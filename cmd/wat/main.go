package main

import (
	"fmt"
	"os"

	"github.com/jmacd/caspar.water/cmd/internal/wat"
	"github.com/spf13/cobra"
)

func command() *cobra.Command {
	var config string
	var books *wat.Books
	root := &cobra.Command{
		Use:           "wat",
		Short:         "Water Accounting Terminal - local hledger bookkeeping",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := wat.LoadConfig(config)
			if err != nil {
				return err
			}
			books, err = wat.Open(cfg)
			return err
		},
		RunE: func(_ *cobra.Command, _ []string) error { return wat.RunTerminal(books) },
	}
	root.PersistentFlags().StringVar(&config, "config", "wat.yaml", "workspace YAML configuration")
	root.AddCommand(&cobra.Command{
		Use:   "check",
		Short: "Validate the complete books with hledger strict checks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := books.Check(); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Books pass hledger strict checks (not a reconciliation or completeness check).")
			return nil
		},
	}, &cobra.Command{
		Use:   "report NAME",
		Short: "Show an hledger report",
		Long:  "Reports: " + wat.ReportNames(),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			text, err := books.Report(args[0])
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), text)
			return err
		},
	})
	return root
}

func main() {
	if err := command().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "wat:", err)
		os.Exit(1)
	}
}
