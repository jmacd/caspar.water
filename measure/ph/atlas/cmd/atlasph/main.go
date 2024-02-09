package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/jmacd/caspar.water/measure/ph/atlas/internal/device"
	"github.com/jmacd/caspar.water/measure/ph/atlas/internal/ezo"
	"github.com/spf13/cobra"
)

const defaultAddress = 0x63
const defaultDevice = "/dev/i2c-2"
const defaultTemp = "15C"

var (
	rootCmd = &cobra.Command{
		Use:   "atlasph",
		Short: "Interacts with Atlas pH receiver",
		Long:  "Interacts with Atlas pH receiver",
	}

	flagAddress = rootCmd.PersistentFlags().IntP("i2c_addr", "i", defaultAddress, "i2c address")
	flagDevice  = rootCmd.PersistentFlags().StringP("i2c_device", "d", defaultDevice, "i2c device")

	canceled = fmt.Errorf("operation canceled by user")
	reader   = bufio.NewReader(os.Stdin)
)

func init() {
	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(nameCmd)
	rootCmd.AddCommand(calCmd)
	rootCmd.AddCommand(clearCmd)
}

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Print device information",
	Args:  cobra.NoArgs,
	RunE:  runInfo,
}

var nameCmd = &cobra.Command{
	Use:   "set_name",
	Short: "Set device name",
	Args:  cobra.ExactArgs(1),
	RunE:  runSetName,
}

var calCmd = &cobra.Command{
	Use:   "calibrate",
	Short: "Perform 3-point calibration",
	Args:  cobra.NoArgs,
	RunE:  runCal,
}

var clearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear calibration",
	Args:  cobra.NoArgs,
	RunE:  runClear,
}

func main() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func opener() (*ezo.Ph, error) {
	pdev, err := device.New(*flagDevice, *flagAddress)
	if err != nil {
		return nil, fmt.Errorf("Open Atlas pH: %w", err)
	}
	return ezo.New(pdev), nil
}

func show(ph *ezo.Ph) error {
	info, err := ph.Info()
	if err != nil {
		return fmt.Errorf("Atlas pH info: %w", err)
	}
	status, err := ph.Status()
	if err != nil {
		return fmt.Errorf("Atlas pH status: %w", err)
	}
	name, err := ph.Name()
	if err != nil {
		return fmt.Errorf("Atlas pH name: %w", err)
	}
	pts, err := ph.CalibrationPoints()
	if err != nil {
		return fmt.Errorf("Atlas pH calibration points: %w", err)
	}
	acid, base, offset, err := ph.Slope()
	if err != nil {
		return fmt.Errorf("Atlas pH slope: %w", err)
	}
	fmt.Println("Info: name", name)
	fmt.Println("Info: version", info.Version)
	fmt.Println("Info: restart", status.Restart)
	fmt.Println("Info: Vcc", status.Vcc)
	fmt.Println("Info: calibration", ezo.ExpandCalibrationPoints(pts))
	if pts == 3 {
		fmt.Printf("Info: slope, acid: %.1f%%\n", acid)
		fmt.Printf("Info: slope, base: %.1f%%\n", base)
		fmt.Printf("Info: slope, offset: %.2fmV\n", offset)
	}
	return nil
}

func runInfo(cmd *cobra.Command, _ []string) error {
	ph, err := opener()
	if err != nil {
		return err
	}
	defer ph.Close()
	if err = show(ph); err != nil {
		return err
	}
	return nil
}

func runSetName(cmd *cobra.Command, args []string) error {
	ph, err := opener()
	if err != nil {
		return err
	}
	defer ph.Close()

	name := args[0]
	err = ph.SetName(name)
	if err != nil {
		return fmt.Errorf("Atlas pH name: %w", err)
	}

	nname, err := ph.Name()
	if err != nil {
		return fmt.Errorf("Atlas pH name: %w", err)
	}
	if nname != name {
		return fmt.Errorf("Atlas pH set_name failed")
	}
	return nil
}

func say(msg string, args ...any) {
	fmt.Printf(msg+"\n", args...)
}

func askBool(def string) (bool, error) {
	ln, _, err := reader.ReadLine()
	if err != nil {
		return false, err
	}
	lns := string(ln)
	if lns == "" {
		lns = def
	}
	return strings.ToLower(lns) == "y", nil
}

func askYes(prompt string) (bool, error) {
	fmt.Print(prompt + " [Yn] ")
	return askBool("y")
}

func askNo(prompt string) (bool, error) {
	fmt.Print(prompt + " [yN] ")
	return askBool("n")
}

func askString(prompt, def string) (string, error) {
	fmt.Print(prompt + "  [" + def + "] ")
	ln, _, err := reader.ReadLine()
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
	c := strings.HasSuffix(s, "c")
	f := strings.HasSuffix(s, "f")
	if !c && !f {
		return 0, fmt.Errorf("expected temperature with C or F suffix")
	}
	x, err := strconv.ParseFloat(s[:len(s)-1], 64)
	if err != nil {
		return 0, err
	}
	if c {
		return x, nil
	}
	return (x - 32) / 1.8, nil
}

func runCal(cmd *cobra.Command, _ []string) error {
	ph, err := opener()
	if err != nil {
		return err
	}
	defer ph.Close()
	if err = show(ph); err != nil {
		return err
	}
	pts, err := ph.CalibrationPoints()
	if err != nil {
		return err
	}
	switch pts {
	case 3:
		if ok, err := askYes("device has been calibrated; clear and continue?"); err != nil {
			return err
		} else if !ok {
			return canceled
		}
		err = ph.ClearCalibration()
		if err != nil {
			return err
		}
		pts = 0
	case 2, 1:
		if ok, err := askNo("device has in-progress calibration points; continue?"); err != nil {
			return err
		} else if !ok {
			return canceled
		}
	}
	refTempStr, err := askString("enter the temperature to use for calibration", defaultTemp)
	if err != nil {
		return err
	}
	refTempC, err := parseTemp(refTempStr)
	if err != nil {
		return err
	}
	say("reference temp is %.2fC", refTempC)

	pointName := []string{"mid", "low", "high"}
	phValues := []float64{7, 4, 10}
	for {
		pts, err := ph.CalibrationPoints()
		if err != nil {
			return err
		}
		if pts == 3 {
			break
		}
		say("next calibration point - %s", pointName[pts])

		refPh, err := askString("enter pH value", fmt.Sprintf("%.2f", phValues[pts]))
		if err != nil {
			return err
		}
		refPhF, err := strconv.ParseFloat(refPh, 64)
		if err != nil {
			return err
		}

		say("place probe into ph %.2f reference solution", refPhF)
		say("wait for reading to stabilize and press any key")

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
				reading, err := ph.ReadPh(refTempC)
				if err != nil {
					errCh <- err
					return
				}

				valueCh <- reading
			}
		}()

		go func() {
			defer wait.Done()
			_, _, err = reader.ReadRune()
			if err != nil {
				errCh <- err
				return
			}
			doneCh <- struct{}{}
		}()
		for {
			select {
			case pval := <-valueCh:
				say("%.2f", pval)
				continue
			case <-doneCh:
				close(doneCh)
			case err := <-errCh:
				say("error encountered, aborting")
				return err
			}
			break
		}

		wait.Wait()

		switch pts {
		case 0:
			err = ph.CalibrateMidpoint(refPhF)
		case 1:
			err = ph.CalibrateLowpoint(refPhF)
		case 2:
			err = ph.CalibrateHighpoint(refPhF)
		}
		if err != nil {
			return err
		}
		say("set calibration point - %s", pointName[pts])
	}

	acid, base, offset, err := ph.Slope()
	if err != nil {
		return err
	}
	fmt.Printf("Info: Acid slope %.2f%%\n", acid)
	fmt.Printf("Info: Base slope %.2f%%\n", base)
	fmt.Printf("Info: Offset %.2fmV\n", offset)
	return nil
}

func runClear(cmd *cobra.Command, _ []string) error {
	ph, err := opener()
	if err != nil {
		return err
	}
	defer ph.Close()
	return ph.ClearCalibration()
}
