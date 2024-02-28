package calibrate

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/jmacd/caspar.water/measure/ph/atlasph/internal/ezo"
	"golang.org/x/sync/errgroup"
)

const DefaultTemp = "15C"

type Calibration struct {
	rdr Interactive
	ph  *ezo.Ph
}

type Interactive interface {
	ReadRune() (r rune, size int, err error)
	ReadLine() (line []byte, isPrefix bool, err error)
}

var canceled = fmt.Errorf("operation canceled by user")

func New(rdr Interactive, ph *ezo.Ph) *Calibration {
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

func (c *Calibration) askYes(prompt string, args ...any) (bool, error) {
	fmt.Printf(prompt+" [Yn] ", args...)
	return c.askBool("y")
}

func (c *Calibration) askNo(prompt string, args ...any) (bool, error) {
	fmt.Printf(prompt+" [yN] ", args...)
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

func (c *Calibration) AskTemp() (C float64, _ error) {
	refTempStr, err := c.askString("enter the temperature to use for calibration", DefaultTemp)
	if err != nil {
		return 0, err
	}
	return parseTemp(refTempStr)
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

func (c *Calibration) Calibrate(calPoints int) error {
	pts, err := c.ph.CalibrationPoints()
	if err != nil {
		return err
	}
	if pts >= calPoints {
		if ok, err := c.askYes("device %d-point calibration; clear and continue?", pts); err != nil {
			return err
		} else if !ok {
			return canceled
		}
		err = c.ph.ClearCalibration()
		if err != nil {
			return err
		}
		pts = 0
	}
	if pts != 0 {
		if ok, err := c.askNo("device has in-progress calibration points; continue?"); err != nil {
			return err
		} else if !ok {
			return canceled
		}
	}
	refTempC, err := c.AskTemp()
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
		if pts == calPoints {
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

		c.say("place probe into pH %.2f reference solution", refPhF)
		g, ctx := errgroup.WithContext(context.Background())

		valueCh := make(chan float64)

		g.Go(func() error {
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
				reading, err := c.ph.ReadPh(refTempC)
				if err != nil {
					return err
				}

				select {
				case valueCh <- reading:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		})

		select {
		case <-ctx.Done():
			return ctx.Err()
		case pval := <-valueCh:
			c.say("wait for reading to stabilize and press any key")
			c.say("reading: %.2f", pval)
		}
		g.Go(func() error {
			_, _, err = c.rdr.ReadRune()
			if err != nil {
				return err
			}
			return canceled
		})
		g.Go(func() error {
			for {
				select {
				case pval := <-valueCh:
					c.say("reading: %.2f", pval)
					continue
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		})

		if err := g.Wait(); err != canceled {
			return err
		}

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
	fmt.Printf("Acid slope %.1f%%\n", acid)
	fmt.Printf("Base slope %.1f%%\n", base)
	fmt.Printf("Neutral offset %.2fmV\n", offset)
	return nil
}
