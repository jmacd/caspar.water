// There are a lot of bme280 libraries out there, here's another.
//
// Reference:
// https://www.bosch-sensortec.com/media/boschsensortec/downloads/datasheets/bst-bme280-ds002.pdf

package bme280

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"

	"golang.org/x/exp/io/i2c"
)

// Memory map
const (
	// Control registers
	MM_ID_Reg            = 0xD0
	MM_Ctrl_Humidity_Reg = 0xF2
	MM_Status_Reg        = 0xF3
	MM_Ctrl_Measure_Reg  = 0xF4

	// Unused registers
	// MM_Reset_Reg = 0xE0
	// MM_Config_Reg = 0xF5

	// Calibration registers
	MM_Calib_T1_T3_Regs = 0x88 // T1 - T3
	MM_Calib_P1_P9_Regs = 0x8E // P1 - P9
	MM_Calib_H1_Reg     = 0xA1 // H1
	MM_Calib_H2_H6_Regs = 0xE1 // H2 - H6

	// Data registers
	MM_Pressure_Data_Reg    = 0xF7
	MM_Temperature_Data_Reg = 0xFA
	MM_Humidity_Data_Reg    = 0xFD
)

type Accuracy int

const (
	UltraLowAccuracy Accuracy = iota
	LowAccuracy
	StandardAccuracy
	HighAccuracy
	UltraHighAccuracy
)

// forceModeCtrlMeasure indicates forced mode in the Ctrl_Measure register byte.
const forceModeCtrlMeasure byte = 1

type BME280 struct {
	device *i2c.Device
	chipID byte
	acc    Accuracy
	trim   TrimmingParameters
}

// uncompensated contains raw analog-to-digital readings.
type uncompensated struct {
	P int32 // 20 bits
	T int32 // 20 bits
	H int32 // 16 bits
}

// Measurements contains compensated measurement values.
type Measurements struct {
	T float64 // unit: C
	P float64 // unit: Pa
	H float64 // unit: RH%
}

// TrimmingParameters for the bme280 §4.2.2
type TrimmingParameters struct {
	// Signed/unsigned and type widths are given in table 16.

	T1 uint16
	T2 int16
	T3 int16

	P1 uint16
	P2 int16
	P3 int16
	P4 int16
	P5 int16
	P6 int16
	P7 int16
	P8 int16
	P9 int16

	H1 uint8
	H2 int16
	H3 uint8
	H4 int16
	H5 int16
	H6 int8
}

func New(i2cPath string, devAddr int, acc Accuracy) (*BME280, error) {
	opener := &i2c.Devfs{
		Dev: i2cPath,
	}
	device, err := i2c.Open(opener, devAddr)
	if err != nil {
		return nil, err
	}
	var chipID [1]byte
	if err = device.ReadReg(MM_ID_Reg, chipID[:]); err != nil {
		return nil, err
	}
	switch chipID[0] {
	case 0x56, 0x57, 0x58: // a BMP280
	case 0x60: // a BME280
	case 0x61: // a BME680
	default:
		return nil, fmt.Errorf("unrecognized sensor chip ID: %x", chipID[0])
	}

	bme := &BME280{
		device: device,
		chipID: chipID[0],
		acc:    acc,
	}
	return bme, bme.readTrim()
}

func (bme *BME280) Close() error {
	return bme.device.Close()
}

// The trim parameters accessors below return in the type used in the
// compensation formulas.

func (bme *BME280) T1() int32 { return int32(bme.trim.T1) }
func (bme *BME280) T2() int32 { return int32(bme.trim.T2) }
func (bme *BME280) T3() int32 { return int32(bme.trim.T3) }

func (bme *BME280) P1() int64 { return int64(bme.trim.P1) }
func (bme *BME280) P2() int64 { return int64(bme.trim.P2) }
func (bme *BME280) P3() int64 { return int64(bme.trim.P3) }
func (bme *BME280) P4() int64 { return int64(bme.trim.P4) }
func (bme *BME280) P5() int64 { return int64(bme.trim.P5) }
func (bme *BME280) P6() int64 { return int64(bme.trim.P6) }
func (bme *BME280) P7() int64 { return int64(bme.trim.P7) }
func (bme *BME280) P8() int64 { return int64(bme.trim.P8) }
func (bme *BME280) P9() int64 { return int64(bme.trim.P9) }

func (bme *BME280) H1() int32 { return int32(bme.trim.H1) }
func (bme *BME280) H2() int32 { return int32(bme.trim.H2) }
func (bme *BME280) H3() int32 { return int32(bme.trim.H3) }
func (bme *BME280) H4() int32 { return int32(bme.trim.H4) }
func (bme *BME280) H5() int32 { return int32(bme.trim.H5) }
func (bme *BME280) H6() int32 { return int32(bme.trim.H6) }

func (bme *BME280) Read() (Measurements, error) {
	uncomp, err := bme.readUncompensated()

	var meas Measurements

	// tFine is carried from the temperature calculation to the pressure and
	// humidity calculation.
	var tFine int32
	tFine, meas.T = bme.compTemerature(uncomp.T)
	meas.P = bme.compPressure(tFine, uncomp.P)
	meas.H = bme.compHumidity(tFine, uncomp.H)

	fmt.Printf("T compensated is %.3f C\n", meas.T)
	fmt.Printf("P compensated is %.1f Pa\n", meas.P)
	fmt.Printf("H compensated is %.3f RH%%\n", meas.H)

	return meas, err
}

func (bme *BME280) getOsr() byte {
	switch bme.acc {
	case LowAccuracy:
		return 2
	case StandardAccuracy:
		return 3
	case HighAccuracy:
		return 4
	case UltraHighAccuracy:
		return 5
	default:
		return 1
	}
}

func (bme *BME280) wait() error {
	for n := 0; n < 30; n++ {
		var status [1]byte
		if err := bme.device.ReadReg(MM_Status_Reg, status[:]); err != nil {
			return err
		}
		if status[0]&0x8 == 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	return nil
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func (bme *BME280) readTrim() error {
	var trimT [6]byte
	var trimP [18]byte
	var trimH1 [1]byte
	var trimH2 [8]byte

	// The BME T and P parameters are actually contiguous, but reading them
	// in logical groups, first T parameters.
	if err := bme.device.ReadReg(MM_Calib_T1_T3_Regs, trimT[:]); err != nil {
		return err
	}
	buf := bytes.NewReader(trimT[:])
	check(binary.Read(buf, binary.LittleEndian, &bme.trim.T1))
	check(binary.Read(buf, binary.LittleEndian, &bme.trim.T2))
	check(binary.Read(buf, binary.LittleEndian, &bme.trim.T3))

	// Next, P parameters.
	if err := bme.device.ReadReg(MM_Calib_P1_P9_Regs, trimP[:]); err != nil {
		return err
	}
	buf = bytes.NewReader(trimP[:])
	check(binary.Read(buf, binary.LittleEndian, &bme.trim.P1))
	check(binary.Read(buf, binary.LittleEndian, &bme.trim.P2))
	check(binary.Read(buf, binary.LittleEndian, &bme.trim.P3))
	check(binary.Read(buf, binary.LittleEndian, &bme.trim.P4))
	check(binary.Read(buf, binary.LittleEndian, &bme.trim.P5))
	check(binary.Read(buf, binary.LittleEndian, &bme.trim.P6))
	check(binary.Read(buf, binary.LittleEndian, &bme.trim.P7))
	check(binary.Read(buf, binary.LittleEndian, &bme.trim.P8))
	check(binary.Read(buf, binary.LittleEndian, &bme.trim.P9))

	// BME280 H1 trim parameter is not contiguous with the remaining 7 bytes
	// of H2-H5.  Read the first range.
	if err := bme.device.ReadReg(MM_Calib_H1_Reg, trimH1[:]); err != nil {
		return err
	}
	buf = bytes.NewReader(trimH1[:])
	binary.Read(buf, binary.LittleEndian, &bme.trim.H1)
	// Here we read the second contiguous range.
	if err := bme.device.ReadReg(MM_Calib_H2_H6_Regs, trimH2[:]); err != nil {
		return err
	}
	buf = bytes.NewReader(trimH2[:])
	check(binary.Read(buf, binary.LittleEndian, &bme.trim.H2))
	check(binary.Read(buf, binary.LittleEndian, &bme.trim.H3))

	// H4 and H5 are packed into three bytes.
	var a, b, c uint8
	check(binary.Read(buf, binary.LittleEndian, &a))
	check(binary.Read(buf, binary.LittleEndian, &b))
	check(binary.Read(buf, binary.LittleEndian, &c))

	bme.trim.H4 = int16(a)<<4 | int16(b)&0xf
	bme.trim.H5 = int16(c)<<4 | ((int16(b) & 0xf0) >> 4)

	check(binary.Read(buf, binary.LittleEndian, &bme.trim.H6))

	return nil
}

func (bme *BME280) readUncompensated() (uncomp uncompensated, _ error) {
	var pressure [3]byte
	var temperature [3]byte
	var humidity [2]byte

	// Note: using the same OSR for all three measurements.
	osr := bme.getOsr()

	// The operating mode is set by writing to Measure_Reg which
	// is compatible with BMP280 chips.
	if err := bme.device.WriteReg(MM_Ctrl_Measure_Reg, []byte{
		forceModeCtrlMeasure | (osr << 2) | (osr << 5),
	}); err != nil {
		return uncomp, err
	}

	bme.wait()

	if err := bme.device.ReadReg(MM_Pressure_Data_Reg, pressure[:]); err != nil {
		return uncomp, err
	}

	if err := bme.device.ReadReg(MM_Temperature_Data_Reg, temperature[:]); err != nil {
		return uncomp, err
	}

	// To read humidity on the BME280--this uses the operating mode
	// set above, and the Ctrl_Measure register must be set first.
	if err := bme.device.WriteReg(MM_Ctrl_Humidity_Reg, []byte{osr}); err != nil {
		return uncomp, err
	}

	bme.wait()

	if err := bme.device.ReadReg(MM_Humidity_Data_Reg, humidity[:]); err != nil {
		return uncomp, err
	}

	// pvalue and tvalue are 20 bits; hvalue is 16 bits; these registers are
	// arranged in big-endian order; the MSB has the lowest address, then
	// LSB, then (for p and t), and the 4-bit "XLSB" has the highest address
	// of the set.
	pvalue := int32(pressure[0])<<12 + int32(pressure[1])<<4 + int32(pressure[2]&0xf0)>>4
	tvalue := int32(temperature[0])<<12 + int32(temperature[1])<<4 + int32(temperature[2]&0xf0)>>4
	hvalue := int32(humidity[0])<<8 + int32(humidity[1])

	return uncompensated{
		P: pvalue,
		T: tvalue,
		H: hvalue,
	}, nil
}

// Compensated temperature, from datasheet §4.2.3
func (bme *BME280) compTemerature(adcT int32) (tFine int32, celsius float64) {
	var var1, var2 int32

	var1 = (((adcT >> 3) - (bme.T1() << 1)) * bme.T2()) >> 11
	var2 = (((((adcT >> 4) - bme.T1()) * ((adcT >> 4) - bme.T1())) >> 12) * bme.T3()) >> 14

	// tFine is used in P and H calculations.
	tFine = var1 + var2
	// The formula for temperature yields 1/100 Celsius units, divide by 100
	c100 := (tFine*5 + 128) >> 8
	celsius = float64(c100) / 100
	return
}

// Compensated pressure, from datasheet §4.2.3
func (bme *BME280) compPressure(tFine, adcP int32) (pascals float64) {
	var var1, var2, p int64

	var1 = int64(tFine) - 128000
	var2 = var1 * var1 * bme.P6()
	var2 += (var1 * bme.P5()) << 17
	var2 += bme.P4() << 35
	var1 = ((var1 * var1 * bme.P3()) >> 8) + ((var1 * bme.P2()) << 12)
	var1 = (((int64(1) << 47) + var1) * bme.P1()) >> 33

	if var1 == 0 {
		return 0
	}

	p = 1048576 - int64(adcP)
	p = (((p << 31) - var2) * 3125) / var1
	var1 = (bme.P9() * (p >> 13) * (p >> 13)) >> 25
	var2 = (bme.P8() * p) >> 19
	p = ((p + var1 + var2) >> 8) + (bme.P7() << 4)

	// The formula for pressure yields 1/256 Pa units, divide by 256
	return float64(p) / 256
}

// Compensated humidity, from datasheet §4.2.3
func (bme *BME280) compHumidity(tFine, adcH int32) (relativePct float64) {
	var var1 int32 // a.k.a. v_x1_u32r

	var1 = tFine - 76800
	var1 = (((((adcH << 14) - ((bme.H4()) << 20) - ((bme.H5()) * var1)) + (16384)) >> 15) * (((((((var1*(bme.H6()))>>10)*(((var1*(bme.H3()))>>11)+(32768)))>>10)+(2097152))*(bme.H2()) + 8192) >> 14))
	var1 -= (((((var1 >> 15) * (var1 >> 15)) >> 7) * bme.H1()) >> 4)

	switch {
	case var1 < 0:
		var1 = 0
	case var1 > 419430400:
		var1 = 419430400
	}
	// The formula for humidity yields 1/1024 relative humidity in perecent.
	return float64(var1>>12) / 1024
}
