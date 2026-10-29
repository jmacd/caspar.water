package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/jmacd/caspar.water/cmd/internal/wat"
	"github.com/spf13/cobra"
)

func command() *cobra.Command {
	var config string
	var books *wat.Books
	var statement, statementInbox, extraction string
	var statements bool
	var statementCheck bool
	var quickBooksYear int
	var quickBooksInspect bool
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
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := wat.LoadConfig(config)
			if err != nil {
				return err
			}
			books, err = wat.Open(cfg)
			if err != nil {
				return err
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("statement") && cmd.Flags().Changed("statements") {
				return fmt.Errorf("use either --statement PDF or --statements PDF..., not both")
			}
			var paths []string
			if statements {
				if extraction != "" {
					return fmt.Errorf("--statement-extraction requires a single --statement PDF")
				}
				if err := checkStatementFiles(args); err != nil {
					return err
				}
				paths = args
			} else if statement != "" {
				paths = []string{statement}
			}
			if len(paths) == 0 {
				if extraction != "" || statementCheck {
					return fmt.Errorf("statement options require --statement PDF or --statements PDF...")
				}
				if statementInbox != "" {
					if err := books.ImportStatementInbox(statementInbox, cmd.ErrOrStderr()); err != nil {
						return err
					}
				}
				return wat.RunTerminal(books)
			}
			if statements && !statementCheck {
				return wat.RunStatementsTerminal(books, paths, cmd.ErrOrStderr())
			}
			for _, path := range paths {
				s, err := books.LoadStatement(path, wat.StatementOptions{Extraction: extraction}, cmd.ErrOrStderr())
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
	root.PersistentFlags().StringVar(&statementInbox, "statement-inbox", "", "read-only import inbox for statement PDFs and QuickBooks files")
	root.Flags().StringVar(&statement, "statement", "", "extract or resume review of a bank statement PDF")
	root.Flags().BoolVar(&statements, "statements", false, "review PDF file arguments in supplied order, one at a time")
	root.Flags().StringVar(&extraction, "statement-extraction", "", "seed a new statement cache from reviewed bankstatementparser JSON")
	root.Flags().BoolVar(&statementCheck, "statement-check", false, "extract/validate a statement and print progress without opening the terminal")
	root.AddCommand(&cobra.Command{
		Use:   "check",
		Short: "Validate the complete books and posting account depth",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := books.Check(); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Books pass hledger strict checks and CPUC account-mapping checks (not a reconciliation or completeness check).")
			return nil
		},
	}, &cobra.Command{
		Use:   "accounts",
		Short: "List local posting accounts and their formal CPUC designations",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			accounts, err := books.PostingAccounts()
			if err != nil {
				return err
			}
			for _, account := range accounts {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s:%s-%s\n", account.Name, account.FormalCategory, account.Number, account.FormalName)
			}
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
	quickBooks := &cobra.Command{
		Use:   "quickbooks [FILE...]",
		Short: "Generate an editable journal from QuickBooks reports",
		Long: "Generate a balanced candidate journal without changing the books. " +
			"With no FILE arguments, scan the configured read-only inbox for .xls, .xlsx, and .csv files.",
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := quickBooksFiles(args, statementInbox)
			if err != nil {
				return err
			}
			if quickBooksInspect {
				return books.InspectQuickBooks(
					cmd.Context(),
					paths,
					quickBooksYear,
					cmd.OutOrStdout(),
					cmd.ErrOrStderr(),
				)
			}
			journal, err := books.GenerateQuickBooksJournal(
				cmd.Context(), paths, quickBooksYear, cmd.ErrOrStderr(),
			)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), strings.TrimRight(journal, "\n"))
			return err
		},
	}
	quickBooks.Flags().IntVar(&quickBooksYear, "year", 0, "four-digit reconstruction year")
	quickBooks.Flags().BoolVar(
		&quickBooksInspect,
		"inspect",
		false,
		"report deterministic extraction diagnostics without calling a model",
	)
	if err := quickBooks.MarkFlagRequired("year"); err != nil {
		panic(err)
	}
	root.AddCommand(quickBooks)
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

func quickBooksFiles(arguments []string, inbox string) ([]string, error) {
	paths := append([]string(nil), arguments...)
	if len(paths) == 0 {
		if strings.TrimSpace(inbox) == "" {
			return nil, fmt.Errorf("provide QuickBooks files or configure --statement-inbox")
		}
		entries, err := os.ReadDir(inbox)
		if err != nil {
			return nil, fmt.Errorf("read QuickBooks inbox: %w", err)
		}
		for _, entry := range entries {
			if entry.Type().IsRegular() && quickBooksExtension(entry.Name()) {
				paths = append(paths, filepath.Join(inbox, entry.Name()))
			}
		}
		if len(paths) == 0 {
			return nil, fmt.Errorf("QuickBooks inbox contains no .xls, .xlsx, or .csv files")
		}
	} else if strings.TrimSpace(inbox) != "" {
		for index, path := range paths {
			if !filepath.IsAbs(path) {
				paths[index] = filepath.Join(inbox, path)
			}
		}
	}
	for _, path := range paths {
		if !quickBooksExtension(path) {
			return nil, fmt.Errorf("QuickBooks input must be .xls, .xlsx, or .csv: %s", path)
		}
	}
	slices.Sort(paths)
	return paths, nil
}

func quickBooksExtension(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".xls", ".xlsx", ".csv":
		return true
	default:
		return false
	}
}

func main() {
	if err := command().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "wat:", err)
		os.Exit(1)
	}
}
