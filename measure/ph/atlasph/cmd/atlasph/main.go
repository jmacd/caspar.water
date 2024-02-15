package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/jmacd/caspar.water/measure/ph/atlasph/internal/calibrate"
	"github.com/jmacd/caspar.water/measure/ph/atlasph/internal/device"
	"github.com/jmacd/caspar.water/measure/ph/atlasph/internal/ezo"
	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{
		Use:   "atlasph",
		Short: "Interacts with Atlas pH receiver",
		Long:  "Interacts with Atlas pH receiver",
	}

	flagAddress = rootCmd.PersistentFlags().IntP("i2c_addr", "i", ezo.DefaultAddress, "i2c address")
	flagDevice  = rootCmd.PersistentFlags().StringP("i2c_device", "d", ezo.DefaultDevice, "i2c device")
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

func runCal(cmd *cobra.Command, _ []string) error {
	ph, err := opener()
	if err != nil {
		return err
	}
	defer ph.Close()
	if err = show(ph); err != nil {
		return err
	}
	cc := calibrate.NewCalibration(bufio.NewReader(os.Stdin), ph)
	return cc.Calibrate()
}

func runClear(cmd *cobra.Command, _ []string) error {
	ph, err := opener()
	if err != nil {
		return err
	}
	defer ph.Close()
	return ph.ClearCalibration()
}
