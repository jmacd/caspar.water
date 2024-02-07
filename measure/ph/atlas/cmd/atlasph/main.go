package main

import (
	"fmt"
	"os"
	"sync"

	"github.com/jmacd/caspar.water/measure/ph/atlas"
	"github.com/jmacd/caspar.water/measure/ph/atlas/internal/ezo"
	"github.com/spf13/cobra"
)

const defaultAddress = 0x63
const defaultDevice = "/dev/i2c-2"

var (
	rootCmd = &cobra.Command{
		Use:   "atlasph",
		Short: "Interacts with Atlas pH receiver",
		Long:  "Interacts with Atlas pH receiver",
	}

	flagAddress = rootCmd.PersistentFlags().IntP("i2c_addr", "i", defaultAddress, "i2c address")
	flagDevice  = rootCmd.PersistentFlags().StringP("i2c_device", "d", defaultDevice, "i2c device")

	canceled = fmt.Errorf("operation canceled by user")
)

func init() {
	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(nameCmd)
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

func main() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func opener() (*ezo.Ph, error) {
	ph, err := atlas.New(*flagDevice, *flagAddress)
	if err != nil {
		return nil, fmt.Errorf("Open Atlas pH: %w", err)
	}
	return ph, nil
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
	fmt.Println("Info: calibration", atlas.ExpandCalibrationPoints(pts))
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

func askYes(prompt string) bool {
	fmt.Println(prompt + " [Yn]")
}

func askNo(prompt string) bool {
	fmt.Println(prompt + " [yN]")
}

func askString(prompt string) string {
	fmt.Println(prompt + " [yN]")
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
		if !askYes("device has been calibrated; clear and continue?") {
			return canceled
		}
		err = ph.ClearCalibration()
		if err != nil {
			return err
		}
		pts = 0
	case 2, 1:
		if !askNo("device has in-progress calibration points; continue?") {
			return canceled
		}
	}
	refTempStr := askString("enter the temperature to use for calibration [%.2f]:")
	refTempC := parseTemp(refTempStr)

	pointName := []string{"mid", "low", "high"}
	phValues := []float64{7, 4, 10}
	for pts < 3 {
		say("next calibration point - %s", pointName[pts])

		refPh := ask("enter pH value [%.2f]", phValues[pts])

		say("place probe into ph %.2f reference solution")
		say("wait for reading to stabilize and press any key")

		var wait sync.WaitGroup
		wait.Add(2)

		errCh := make(chan error)
		doneCh := make(chan struct{})
		valueCh := make(chan float64)
		strokeCh := make(chan struct{})

		go func() {
			defer wait.Done()
			for {
				select {
				case <-doneCh:
					return
				}
				reading, err := ph.ReadPh(refTemp)
				if err != nil {
					errCh <- err
					return
				}

				valueCh <- reading
			}
		}()

		for {
			select {
			case pval := <-valueCh:
				say("%.2f", pval)
			case <-strokeCh:
				say("got it")
				// break
			case <-errCh:
				say("error encountered, aborting")
				return //@@
			}
		}

		// Issue Cal,mid,refPh
	}

	return nil
}
