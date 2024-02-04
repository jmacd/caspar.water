// https://files.atlas-scientific.com/pH_EZO_Datasheet.pdf
package atlas

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/exp/io/i2c"
)

const (
	millis = time.Millisecond
	short  = 300 * millis
)

type Ph struct {
	device *i2c.Device
}

type Info struct {
	Version string
}

type Status struct {
	Vcc     float64
	Restart string
}

type Measurements struct {
	Ph float64
}

func New(i2cPath string, devAddr int) (*Ph, error) {
	opener := &i2c.Devfs{
		Dev: i2cPath,
	}
	device, err := i2c.Open(opener, devAddr)
	if err != nil {
		return nil, err
	}
	ph := &Ph{
		device: device,
	}

	// if info, err := ph.readStrings("i", 3, short); err != nil {
	// 	return nil, fmt.Errorf("Device likely not an Atlas EZO pH receiver: %w", err)
	// } else if !expectAtIdx(info, 0, "I") || !expectAtIdx(info, 1, "pH") || len(info) != 3 {
	// 	return nil, fmt.Errorf("Unexpected Info response: %q", info)
	// } else {
	// 	fmt.Println("pH: firmware version", info[2])
	// 	ph.version = info[2]
	// }

	return ph, nil
}

func (ph *Ph) Info() (info Info, _ error) {
	strs, err := ph.readStrings("i", 3, short)
	if err != nil {
		return info, fmt.Errorf("Device likely not an Atlas EZO pH receiver: %w", err)
	} else if !expectAtIdx(strs, 0, "I") || !expectAtIdx(strs, 1, "pH") {
		return info, fmt.Errorf("Unexpected Info response: %q", strs)
	}
	info.Version = strs[2]
	return info, nil
}

func (ph *Ph) Status() (status Status, _ error) {
	strs, err := ph.readStrings("Status", 3, short)
	if err != nil {
		return status, fmt.Errorf("Error reading Status: %w", err)
	} else if !expectAtIdx(strs, 0, "Status") {
		return status, fmt.Errorf("Unexpected Status response: %v", strs)
	}
	status.Restart = ExpandRestartCode(strs[1])
	status.Vcc, err = strconv.ParseFloat(strs[2], 64)
	return status, err
}

func (ph *Ph) Name() (string, error) {
	strs, err := ph.readStrings("Name,?", 2, short)
	if err != nil {
		return "", fmt.Errorf("Error reading name: %w", err)
	} else if !expectAtIdx(strs, 0, "Name") {
		return "", fmt.Errorf("Unexpected Status response: %v", strs)
	}
	return strs[1], nil
}

func (ph *Ph) SetName(name string) error {
	err := ph.readCommand("Name,"+name, short)
	if err != nil {
		return fmt.Errorf("Error saving name: %w", err)
	}
	return nil
}

func ExpandRestartCode(str string) string {
	if len(str) == 1 {
		switch str[0] {
		case 'P':
			return "Powered off"
		case 'S':
			return "Software reset"
		case 'B':
			return "Brown-out"
		case 'W':
			return "Watchdog"
		case 'U':
			return "Unknown"
		}
	}
	return "Unrecognized:" + str
}

// if err := ph.readCommand("L,0", short); err != nil {
// 	return nil, fmt.Errorf("Error setting LED state: %w", err)
// }

func (ph *Ph) Close() error {
	return ph.device.Close()
}

func (ph *Ph) Read() (Measurements, error) {
	return Measurements{}, nil
}

func (ph *Ph) read(cmd string, wait time.Duration) ([]byte, error) {
	if err := ph.device.Write([]byte(cmd)); err != nil {
		return nil, err
	}
	time.Sleep(wait)
	for n := 0; n < 2; n++ {
		var status [40]byte
		if err := ph.device.Read(status[:]); err != nil {
			return nil, err
		}
		switch status[0] {
		case 255:
			// No data
			return nil, fmt.Errorf("No data to read")
		case 254:
			// Processing
			continue
		case 2:
			// Syntax
			return nil, fmt.Errorf("Command syntax error")
		case 1:
			// OK
			data, _, _ := bytes.Cut(status[1:], []byte{0})
			return data, nil
		}
		time.Sleep(5 * time.Millisecond)
	}
	return nil, fmt.Errorf("Timeout")
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
	if len(vals) != num {
		return nil, fmt.Errorf("Expected %d string values: %v", num, vals)
	}
	return vals, nil
}

func expectAtIdx(strs []string, idx int, val string) bool {
	return len(strs) > idx && strings.ToUpper(strs[idx]) == strings.ToUpper(val)
}
