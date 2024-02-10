package calibrate

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/jmacd/caspar.water/measure/ph/atlas/internal/ezo"
)

const DefaultTemp = "15C"

type Calibration struct {
	rdr *bufio.Reader
	ph  *ezo.Ph
}

var canceled = fmt.Errorf("operation canceled by user")

func NewCalibration(rdr *bufio.Reader, ph *ezo.Ph) *Calibration {
	return &Calibration{rdr: rdr, ph: ph}
}

func (c *Calibration) say(msg string, args ...any) {
	fmt.Printf(msg+"\n", args...)
}

func (c *Calibration) askBool(def string) (bool, error) {
	ln, _, err := c.rdr.ReadLine()
	if err != nil {
		return false, err
	}
	lns := string(ln)
	if lns == "" {
		lns = def
	}
	return strings.ToLower(lns) == "y", nil
}

func (c *Calibration) askYes(prompt string) (bool, error) {
	fmt.Print(prompt + " [Yn] ")
	return c.askBool("y")
}

func (c *Calibration) askNo(prompt string) (bool, error) {
	fmt.Print(prompt + " [yN] ")
	return c.askBool("n")
}

func (c *Calibration) askString(prompt, def string) (string, error) {
	fmt.Print(prompt + "  [" + def + "] ")
	ln, _, err := c.rdr.ReadLine()
	if err != nil {
		return "", err
	}
	lns := string(ln)
	if lns == "" {
		lns = def
	}
	return lns, nil
}

// parseTemp returns celsius
func parseTemp(s string) (float64, error) {
	s = strings.ToLower(s)
	hasC := strings.HasSuffix(s, "c")
	hasF := strings.HasSuffix(s, "f")
	if !hasC && !hasF {
		return 0, fmt.Errorf("expected temperature with C or F suffix")
	}
	x, err := strconv.ParseFloat(s[:len(s)-1], 64)
	if err != nil {
		return 0, err
	}
	if hasC {
		return x, nil
	}
	return (x - 32) / 1.8, nil
}

func (c *Calibration) Calibrate() error {
	pts, err := c.ph.CalibrationPoints()
	if err != nil {
		return err
	}
	switch pts {
	case 3:
		if ok, err := c.askYes("device has been calibrated; clear and continue?"); err != nil {
			return err
		} else if !ok {
			return canceled
		}
		err = c.ph.ClearCalibration()
		if err != nil {
			return err
		}
		pts = 0
	case 2, 1:
		if ok, err := c.askNo("device has in-progress calibration points; continue?"); err != nil {
			return err
		} else if !ok {
			return canceled
		}
	}
	refTempStr, err := c.askString("enter the temperature to use for calibration", DefaultTemp)
	if err != nil {
		return err
	}
	refTempC, err := parseTemp(refTempStr)
	if err != nil {
		return err
	}
	c.say("reference temp is %.2fC", refTempC)

	pointName := []string{"mid", "low", "high"}
	phValues := []float64{7, 4, 10}
	for {
		pts, err := c.ph.CalibrationPoints()
		if err != nil {
			return err
		}
		if pts == 3 {
			break
		}
		c.say("next calibration point - %s", pointName[pts])

		refPh, err := c.askString("enter pH value", fmt.Sprintf("%.2f", phValues[pts]))
		if err != nil {
			return err
		}
		refPhF, err := strconv.ParseFloat(refPh, 64)
		if err != nil {
			return err
		}

		c.say("place probe into ph %.2f reference solution", refPhF)
		c.say("wait for reading to stabilize and press any key")

		var wait sync.WaitGroup
		wait.Add(2)

		errCh := make(chan error, 2)
		doneCh := make(chan struct{})
		valueCh := make(chan float64, 1)

		go func() {
			defer wait.Done()
			for {
				select {
				case <-doneCh:
					return
				default:
				}
				reading, err := c.ph.ReadPh(refTempC)
				if err != nil {
					errCh <- err
					return
				}

				valueCh <- reading
			}
		}()

		go func() {
			defer wait.Done()
			_, _, err = c.rdr.ReadRune()
			if err != nil {
				errCh <- err
				return
			}
			doneCh <- struct{}{}
		}()
		for {
			select {
			case pval := <-valueCh:
				c.say("%.2f", pval)
				continue
			case <-doneCh:
				close(doneCh)
			case err := <-errCh:
				c.say("error encountered, aborting")
				return err
			}
			break
		}

		wait.Wait()

		switch pts {
		case 0:
			err = c.ph.CalibrateMidpoint(refPhF)
		case 1:
			err = c.ph.CalibrateLowpoint(refPhF)
		case 2:
			err = c.ph.CalibrateHighpoint(refPhF)
		}
		if err != nil {
			return err
		}
		c.say("set calibration point - %s", pointName[pts])
	}

	acid, base, offset, err := c.ph.Slope()
	if err != nil {
		return err
	}
	fmt.Printf("Info: Acid slope %.2f%%\n", acid)
	fmt.Printf("Info: Base slope %.2f%%\n", base)
	fmt.Printf("Info: Offset %.2fmV\n", offset)
	return nil
}
