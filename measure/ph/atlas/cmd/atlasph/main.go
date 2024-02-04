package main

import (
	"fmt"
	"os"

	"github.com/jmacd/caspar.water/measure/ph/atlas"
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
	Short: "Print device name",
	Args:  cobra.ExactArgs(1),
	RunE:  runSetName,
}

func main() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func runInfo(cmd *cobra.Command, _ []string) error {
	ph, err := atlas.New(*flagDevice, *flagAddress)
	if err != nil {
		return fmt.Errorf("Open Atlas pH: %w", err)
	}
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
	fmt.Println("Info: version", info.Version)
	fmt.Println("Info: restart", status.Restart)
	fmt.Println("Info: Vcc", status.Vcc)
	fmt.Println("Info: name", name)

	return ph.Close()
}

func runSetName(cmd *cobra.Command, args []string) error {
	ph, err := atlas.New(*flagDevice, *flagAddress)
	if err != nil {
		return fmt.Errorf("Open Atlas pH: %w", err)
	}
	err = ph.SetName(args[0])
	if err != nil {
		return fmt.Errorf("Atlas pH name: %w", err)
	}

	return ph.Close()
}
