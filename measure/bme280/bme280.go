//--------------------------------------------------------------------------------------------------
//
// Copyright (c) 2020 zack Wang
//
// Permission is hereby granted, free of charge, to any person obtaining a copy of this software and
// associated documentation files (the "Software"), to deal in the Software without restriction,
// including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense,
// and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so,
// subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all copies or substantial
// portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING
// BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
// NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM,
// DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
//
//--------------------------------------------------------------------------------------------------

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
	MM_ID_Reg    = 0xD0
	MM_Reset_Reg = 0xE0

	MM_Ctrl_Humidity_Reg = 0xF2
	MM_Status_Reg        = 0xF3
	MM_Ctrl_Measure_Reg  = 0xF4
	MM_Config_Reg        = 0xF5

	MM_Calib_T1_T3_Regs = 0x88 // T1 - T3
	MM_Calib_P1_P9_Regs = 0x8E // P1 - P9
	MM_Calib_H1_Reg     = 0xA1 // H1
	MM_Calib_H2_H5_Regs = 0xE1 // H2 - H5

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

type uncompensated struct {
	P, T, H int32
}

type Measurements struct {
	T int32 // in 0.01 deg C

}

// TrimmingParameters for the bme280
type TrimmingParameters struct {
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

func (bme *BME280) Read() (Measurements, error) {
	uncomp, err := bme.readUncompensated()

	fmt.Println("Three ADC values are: ", uncomp.P, uncomp.T, uncomp.H)

	var meas Measurements
	trim := &bme.trim

	// Compensated temperature (4.2.3)
	var1 := (((uncomp.T >> 3) - (int32(trim.T1) << 1)) * (int32(trim.T2))) >> 11
	var2 := (((((uncomp.T >> 4) - (int32(trim.T1))) * ((uncomp.T >> 4) - (int32(trim.T1)))) >> 12) * (int32(trim.T3))) >> 14

	// tFine is used in P and H calculations
	tFine := var1 + var2
	meas.T = (tFine*5 + 128) >> 8

	fmt.Printf("T compensated is %f deg C\n", float64(meas.T)*0.01)

	// Compensated temperature (4.2.3)
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

func (bme *BME280) readTrim() error {
	var trimT [6]byte
	var trimP [18]byte
	var trimH1 [1]byte
	var trimH2 [7]byte

	if err := bme.device.ReadReg(MM_Calib_T1_T3_Regs, trimT[:]); err != nil {
		return err
	}
	buf := bytes.NewReader(trimT[:])
	binary.Read(buf, binary.LittleEndian, &bme.trim.T1)
	binary.Read(buf, binary.LittleEndian, &bme.trim.T2)
	binary.Read(buf, binary.LittleEndian, &bme.trim.T3)

	if err := bme.device.ReadReg(MM_Calib_P1_P9_Regs, trimP[:]); err != nil {
		return err
	}
	buf = bytes.NewReader(trimP[:])
	binary.Read(buf, binary.LittleEndian, &bme.trim.P1)
	binary.Read(buf, binary.LittleEndian, &bme.trim.P2)
	binary.Read(buf, binary.LittleEndian, &bme.trim.P3)
	binary.Read(buf, binary.LittleEndian, &bme.trim.P4)
	binary.Read(buf, binary.LittleEndian, &bme.trim.P5)
	binary.Read(buf, binary.LittleEndian, &bme.trim.P6)
	binary.Read(buf, binary.LittleEndian, &bme.trim.P7)
	binary.Read(buf, binary.LittleEndian, &bme.trim.P8)
	binary.Read(buf, binary.LittleEndian, &bme.trim.P9)

	if err := bme.device.ReadReg(MM_Calib_H1_Reg, trimH1[:]); err != nil {
		return err
	}
	buf = bytes.NewReader(trimH1[:])
	binary.Read(buf, binary.LittleEndian, &bme.trim.H1)

	if err := bme.device.ReadReg(MM_Calib_H2_H5_Regs, trimH2[:]); err != nil {
		return err
	}
	buf = bytes.NewReader(trimH2[:])
	binary.Read(buf, binary.LittleEndian, &bme.trim.H2)
	binary.Read(buf, binary.LittleEndian, &bme.trim.H3)
	binary.Read(buf, binary.LittleEndian, &bme.trim.H4)
	binary.Read(buf, binary.LittleEndian, &bme.trim.H5)

	return nil
}

func (bme *BME280) readUncompensated() (uncomp uncompensated, _ error) {
	var pressure [3]byte
	var temperature [3]byte
	var humidity [2]byte

	// Note: using the same OSR for all three measurements.
	osr := bme.getOsr()

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

	if err := bme.device.WriteReg(MM_Ctrl_Humidity_Reg, []byte{osr}); err != nil {
		return uncomp, err
	}

	bme.wait()

	if err := bme.device.ReadReg(MM_Humidity_Data_Reg, humidity[:]); err != nil {
		return uncomp, err
	}

	pvalue := int32(pressure[0])<<12 + int32(pressure[1])<<4 + int32(pressure[2]&0xf0)>>4
	tvalue := int32(temperature[0])<<12 + int32(temperature[1])<<4 + int32(temperature[2]&0xf0)>>4
	hvalue := int32(humidity[0])<<8 + int32(humidity[1])

	return uncompensated{
		P: pvalue,
		T: tvalue,
		H: hvalue,
	}, nil
}

// // Read Pressure Multple by 10 in unit Pa .
// func ReadPressurePa(d string, a int, accuracy string) (uint32, error) {
// 	adc_T, err := ReadUncompTemprature(d, a, accuracy)
// 	if err != nil {
// 		return 0, err
// 	}
// 	adc_P, err := ReadUncompPressure(d, a, accuracy)
// 	if err != nil {
// 		return 0, err
// 	}
// 	//log.Println("T_ADC=",adc_T,", P_ADC=", adc_P)

// 	err = ReadCoeff(d, a)
// 	if err != nil {
// 		return 0, err
// 	}
// 	//log.Println("Calibration Coeff=",Cal)
// 	//var T int32
// 	var var1, var2, t_fine int32
// 	var p uint32
// 	var1 = (((adc_T >> 3) - (int32(Cal.T1) << 1)) * (int32(Cal.T2))) >> 11
// 	var2 = (((((adc_T >> 4) - (int32(Cal.T1))) * ((adc_T >> 4) - (int32(Cal.T1)))) >> 12) * (int32(Cal.T3))) >> 14
// 	t_fine = var1 + var2
// 	// T = (t_fine * 5 + 128) >> 8
// 	//log.Println("T (fine)=",float32(T)/100)

// 	var1 = ((int32(t_fine)) >> 1) - 64000
// 	var2 = (((var1 >> 2) * (var1 >> 2)) >> 11) * (int32(Cal.P6))
// 	var2 = var2 + ((var1 * (int32(Cal.P5))) << 1)
// 	var2 = (var2 >> 2) + ((int32(Cal.P4)) << 16)
// 	var1 = (((int32(Cal.P3) * (((var1 >> 2) * (var1 >> 2)) >> 13)) >> 3) + (((int32(Cal.P2)) * var1) >> 1)) >> 18
// 	var1 = (((32768 + var1) * (int32(Cal.P1))) >> 15)
// 	if var1 == 0 {
// 		return 0, nil // avoid exception caused by division by zero
// 	}

// 	p = uint32(3125) * uint32((1048576-adc_P)-(var2>>12))
// 	if p < 0x80000000 {
// 		p = (p << 1) / (uint32(var1))
// 	} else {
// 		p = (p / uint32(var1)) * 2
// 	}
// 	var1 = ((int32(Cal.P9)) * (int32(((p >> 3) * (p >> 3)) >> 13))) >> 12
// 	var2 = ((int32((p >> 2))) * (int32(Cal.P8))) >> 13
// 	p = uint32(int32(p) + ((var1 + var2 + int32(Cal.P7)) >> 4))
// 	//log.Println("Pressure=",p)
// 	return p, nil
// }
