package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jmacd/caspar.water/cmd/internal/wat"
	"github.com/spf13/cobra"
)

func command() *cobra.Command {
	var config string
	var books *wat.Books
	var statement, extraction string
	var statements bool
	var statementPages []int
	var statementCheck bool
	root := &cobra.Command{
		Use:           "wat",
		Short:         "Water Accounting Terminal - local hledger bookkeeping",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args: func(cmd *cobra.Command, args []string) error {
			if statements {
				return cobra.MinimumNArgs(1)(cmd, args)
			}
			return cobra.NoArgs(cmd, args)
		},
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := wat.LoadConfig(config)
			if err != nil {
				return err
			}
			books, err = wat.Open(cfg)
			return err
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("statement") && cmd.Flags().Changed("statements") {
				return fmt.Errorf("use either --statement PDF or --statements PDF..., not both")
			}
			var paths []string
			if statements {
				if extraction != "" || len(statementPages) != 0 {
					return fmt.Errorf("--statement-extraction and --statement-pages require a single --statement PDF")
				}
				if err := checkStatementFiles(args); err != nil {
					return err
				}
				paths = args
			} else if statement != "" {
				paths = []string{statement}
			}
			if len(paths) == 0 {
				if extraction != "" || len(statementPages) != 0 || statementCheck {
					return fmt.Errorf("statement options require --statement PDF or --statements PDF...")
				}
				return wat.RunTerminal(books)
			}
			if statements && !statementCheck {
				return wat.RunStatementsTerminal(books, paths, cmd.ErrOrStderr())
			}
			for _, path := range paths {
				s, err := books.LoadStatement(path, wat.StatementOptions{Extraction: extraction, Pages: statementPages}, cmd.ErrOrStderr())
				if err != nil {
					return fmt.Errorf("%s: %w", path, err)
				}
				if !statementCheck {
					return wat.RunStatementTerminal(books, s)
				}
				statuses, err := books.StatementStatuses(s)
				if err != nil {
					return fmt.Errorf("%s: %w", path, err)
				}
				recorded := 0
				for _, status := range statuses {
					if status.Recorded {
						recorded++
					}
				}
				if statements {
					fmt.Fprintln(cmd.OutOrStdout(), "Statement file:", path)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Statement: %d transactions; %d recorded, %d pending. Amounts match statement balances; this is not proof of reconciliation.\n", len(statuses), recorded, len(statuses)-recorded)
				for _, warning := range s.Warnings {
					fmt.Fprintln(cmd.ErrOrStderr(), "Statement notice:", warning)
				}
			}
			return nil
		},
	}
	root.PersistentFlags().StringVar(&config, "config", "wat.yaml", "workspace YAML configuration")
	root.Flags().StringVar(&statement, "statement", "", "extract or resume review of a bank statement PDF")
	root.Flags().BoolVar(&statements, "statements", false, "review PDF file arguments in supplied order, one at a time")
	root.Flags().StringVar(&extraction, "statement-extraction", "", "seed a new statement cache from reviewed bankstatementparser JSON")
	root.Flags().IntSliceVar(&statementPages, "statement-pages", nil, "explicit transaction pages for a new extraction, e.g. 1,2")
	root.Flags().BoolVar(&statementCheck, "statement-check", false, "extract/validate a statement and print progress without opening the terminal")
	root.AddCommand(&cobra.Command{
		Use:   "check",
		Short: "Validate the complete books and posting account depth",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := books.Check(); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Books pass hledger strict checks and account-depth checks (not a reconciliation or completeness check).")
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

func checkStatementFiles(paths []string) error {
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("statement file %q: %w", path, err)
		}
		if !info.Mode().IsRegular() || !strings.EqualFold(filepath.Ext(path), ".pdf") {
			return fmt.Errorf("statement is not a regular PDF file: %s", path)
		}
	}
	return nil
}

func main() {
	if err := command().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "wat:", err)
		os.Exit(1)
	}
}
