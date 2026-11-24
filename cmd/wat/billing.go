package main

import (
	"fmt"

	"github.com/jmacd/caspar.water/cmd/internal/wat"
	"github.com/spf13/cobra"
)

func billingCommand(getBooks func() *wat.Books) *cobra.Command {
	root := &cobra.Command{Use: "billing", Short: "Review pending costs and assess service through a chosen cutoff"}
	var rd, cd wat.BillingDates
	review := &cobra.Command{
		Use: "review [YYYY-MM]", Short: "Inspect expense counts and asset/liability balances over an inclusive period", Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				rd.Month = args[0]
			}
			text, err := getBooks().BillingReview(rd)
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), text)
			return err
		},
	}
	var opt wat.BillingOptions
	closeMonth := &cobra.Command{
		Use: "close [YYYY-MM]", Short: "Preview assessments; --apply TOKEN records the reviewed batch and cutoff", Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("default-weight") && opt.DefaultWeight < 0 {
				return fmt.Errorf("default-weight must be nonnegative")
			}
			if len(args) != 0 {
				cd.Month = args[0]
			}
			text, err := getBooks().BillingClosePeriod(cd, opt)
			if err != nil {
				return err
			}
			if opt.Output != "" {
				_, err = fmt.Fprintln(cmd.OutOrStdout(), "Created separate assessment journal:", opt.Output)
			} else {
				_, err = fmt.Fprint(cmd.OutOrStdout(), text)
			}
			return err
		},
	}
	f := closeMonth.Flags()
	f.StringVar(&opt.Policy, "policy", "", "reviewed billing-policy YAML file")
	f.StringVar(&opt.MarginPercent, "margin-percent", "", "markup on billable costs, e.g. 5.25 for 5.25%")
	f.IntVar(&opt.DefaultWeight, "default-weight", -1, "weight for active connections without an override (otherwise specify each)")
	f.StringArrayVar(&opt.Weights, "weight", nil, "CN-ID=integer weight; repeat; zero excludes a connection")
	f.IntVar(&opt.UnbilledWeight, "unbilled-weight", 0, "additional shares to leave unassigned")
	f.StringVarP(&opt.Output, "output", "o", "", "new .journal outside the workspace (default: preview on stdout)")
	f.StringVar(&opt.Apply, "apply", "", "apply the exact preview by its review token (writes the manual journal)")
	for cmd, dates := range map[*cobra.Command]*wat.BillingDates{review: &rd, closeMonth: &cd} {
		cmd.Flags().StringVar(&dates.From, "from", "", "first included day; defaults to the day after the last applied cutoff")
		cmd.Flags().StringVar(&dates.Through, "through", "", "last included day, YYYY-MM-DD")
	}
	mustRequireFlags(closeMonth, "policy", "margin-percent")
	root.AddCommand(review, closeMonth)
	return root
}
