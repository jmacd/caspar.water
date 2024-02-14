// https://files.atlas-scientific.com/pH_EZO_Datasheet.pdf
package ezo

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jmacd/caspar.water/measure/ph/atlas/internal/device"
)

type Ph struct {
	dev device.I2CStringer
}

type Info struct {
	Version string
}

type Status struct {
	Vcc     float64
	Restart string
}

func New(dev device.I2CStringer) *Ph {
	return &Ph{
		dev: dev,
	}
}

func (ph *Ph) Info() (info Info, _ error) {
	strs, err := ph.readStrings("i", 2, device.Short)
	if err != nil {
		return info, fmt.Errorf("Device likely not an Atlas EZO pH receiver: %w", err)
	} else if !expectAtIdx(strs, 0, "pH") {
		return info, fmt.Errorf("Unexpected Info response: %q", strs)
	}
	info.Version = strs[1]
	return info, nil
}

func (ph *Ph) Status() (status Status, _ error) {
	strs, err := ph.readStrings("Status", 2, device.Short)
	if err != nil {
		return status, fmt.Errorf("Error reading Status: %w", err)
	}
	status.Restart = ExpandRestartCode(strs[0])
	status.Vcc, err = strconv.ParseFloat(strs[1], 64)
	return status, err
}

func (ph *Ph) Name() (string, error) {
	strs, err := ph.readStrings("Name,?", 1, device.Short)
	if err != nil {
		return "", fmt.Errorf("Error reading name: %w", err)
	}
	return strs[0], nil
}

func (ph *Ph) Slope() (acidPct, basePct, offsetMilliVolts float64, _ error) {
	strs, err := ph.readStrings("Slope,?", 3, device.Short)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("Error reading slope: %w", err)
	}
	var fvs [3]float64
	for i, val := range strs {
		fvs[i], err = strconv.ParseFloat(val, 64)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("Parse error in slope: %v: %w", val, err)
		}
	}
	return fvs[0], fvs[1], fvs[2], nil
}

func (ph *Ph) CalibrateMidpoint(statedPh float64) error {
	return ph.calibrateAt("mid", statedPh)
}

func (ph *Ph) CalibrateLowpoint(statedPh float64) error {
	return ph.calibrateAt("low", statedPh)
}

func (ph *Ph) CalibrateHighpoint(statedPh float64) error {
	return ph.calibrateAt("high", statedPh)
}

func (ph *Ph) calibrateAt(pt string, statedPh float64) error {
	return ph.readCommand(fmt.Sprintf("Cal,%s,%.2f", pt, statedPh), device.Long)
}

func (ph *Ph) CalibrationPoints() (points int, _ error) {
	strs, err := ph.readStrings("Cal,?", 1, device.Short)
	if err != nil {
		return 0, fmt.Errorf("Error reading calibration state: %w", err)
	}
	num, err := strconv.Atoi(strs[0])
	if err != nil {
		return 0, err
	}
	if num < 0 || num > 3 {
		return 0, fmt.Errorf("Calibration points: out of range")
	}
	return num, nil
}

func (ph *Ph) ClearCalibration() error {
	return ph.readCommand("Cal,clear", device.Short)
}

func (ph *Ph) SetName(name string) error {
	err := ph.readCommand("Name,"+name, device.Short)
	if err != nil {
		return fmt.Errorf("Error saving name: %w", err)
	}
	return nil
}

func (ph *Ph) ReadPh(tempCelsius float64) (float64, error) {
	cmd := fmt.Sprintf("RT,%.2f", tempCelsius)
	return ph.readFloat(cmd, device.Long)
}

func ExpandCalibrationPoints(num int) string {
	switch num {
	case 0:
		return "uncalibrated, next is mid-point"
	case 1:
		return "in-progress, next is low-point"
	case 2:
		return "in-progress, next is high-point"
	case 3:
		return "calibrated"
	default:
		return "unrecognized"
	}
}

var restartMap = map[byte]string{
	'P': "Powered off",
	'S': "Software reset",
	'B': "Brown-out",
	'W': "Watchdog",
	'U': "Unknown",
}

func ExpandRestartCode(str string) string {
	if len(str) == 1 {
		if reason, ok := restartMap[str[0]]; ok {
			return reason
		}
	}
	return "Unrecognized:" + str
}

func (ph *Ph) Close() error {
	return ph.dev.Close()
}

func (ph *Ph) read(cmd string, wait time.Duration) (string, error) {
	b, s, err := ph.dev.WriteSleepRead(cmd, wait)
	if err != nil {
		return "", err
	}
	switch b {
	case 255:
		// No data
		return "", fmt.Errorf("No data to read")
	case 254:
		// Processing
		return "", fmt.Errorf("Insufficient delay")
	case 2:
		// Syntax
		return "", fmt.Errorf("Command syntax error")
	case 1:
		// OK
		return s, nil
	default:
		return "", fmt.Errorf("Unrecognized command: %s", s)
	}
}

func (ph *Ph) readFloat(cmd string, wait time.Duration) (float64, error) {
	dat, err := ph.read(cmd, wait)
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(string(dat), 64)
}

func (ph *Ph) readCommand(cmd string, wait time.Duration) error {
	dat, err := ph.read(cmd, wait)
	if err != nil {
		return err
	}
	if len(dat) != 0 {
		return fmt.Errorf("Unexpected response data: %v", string(dat))
	}
	return nil
}

func (ph *Ph) readStrings(cmd string, num int, wait time.Duration) ([]string, error) {
	dat, err := ph.read(cmd, wait)
	if err != nil {
		return nil, err
	}
	if len(dat) == 0 || dat[0] != '?' {
		return nil, fmt.Errorf("Missing '?' syntax")
	}
	vals := strings.Split(string(dat[1:]), ",")
	if len(vals) != num+1 {
		return nil, fmt.Errorf("Expected %d string values: %v", num+1, vals)
	}
	// Expect the command-name to echo back, case insensitive.  Split at ',' first:
	if cmdName, _, _ := strings.Cut(cmd, ","); strings.ToUpper(cmdName) != strings.ToUpper(vals[0]) {
		return nil, fmt.Errorf("Unexpected multi-string response syntax: %v != %v", vals[0], cmdName)
	}

	return vals[1:], nil
}

func expectAtIdx(strs []string, idx int, val string) bool {
	return len(strs) > idx && strings.ToUpper(strs[idx]) == strings.ToUpper(val)
}
