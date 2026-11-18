package main

import (
	"fmt"
	"time"

	"github.com/jmacd/caspar.water/cmd/internal/wat"
	"github.com/spf13/cobra"
)

func serviceLifecycleCommands(getBooks func() *wat.Books) []*cobra.Command {
	var commands []*cobra.Command
	for _, action := range []string{"close", "confirm-billing"} {
		var through, date, review string
		cmd := &cobra.Command{
			Use: action + " ACCOUNT", Short: action + " (saves registry only; never moves money)", Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				draft, err := getBooks().NewRegistryDraft()
				if err != nil {
					return err
				}
				if action == "close" {
					err = draft.CloseService(args[0], through)
				} else {
					err = draft.ConfirmServiceBilling(args[0], through, date, review)
				}
				if err != nil {
					return err
				}
				backup, err := draft.Save()
				if err != nil {
					return err
				}
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "Saved %s for %s. No financial posting. Recovery copy: %s\n", action, args[0], backup)
				return err
			},
		}
		cmd.Flags().StringVar(&through, "through", "", "last inclusive service date, YYYY-MM-DD")
		mustRequireFlags(cmd, "through")
		if action == "confirm-billing" {
			cmd.Flags().StringVar(&date, "date", "", "review date, on/after closure and all recorded account activity")
			cmd.Flags().StringVar(&review, "review", "", "explicit billing-completeness review and policy/evidence reference")
			mustRequireFlags(cmd, "date", "review")
		}
		commands = append(commands, cmd)
	}
	var asOf string
	settle := &cobra.Command{
		Use: "settle ACCOUNT", Short: "Review derived settlement status and each obligation; does not move money", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			status, err := getBooks().ServiceSettlement(args[0], asOf)
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), status.String())
			return err
		},
	}
	settle.Flags().StringVar(&asOf, "as-of", time.Now().Format("2006-01-02"), "inclusive review end date")
	commands = append(commands, settle)
	for _, action := range []string{"adjust", "apply", "refund"} {
		var date, through, amount, account, reference, role, transaction string
		var save bool
		cmd := &cobra.Command{
			Use: action + " ACCOUNT", Short: "Preview a reviewed " + action + " step; --save writes one journal", Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				var draft *wat.Draft
				var text string
				var err error
				switch action {
				case "adjust":
					draft, text, err = getBooks().ServiceAdjustmentDraft(args[0], date, through, amount, account, reference)
				case "apply":
					draft, text, err = getBooks().ServiceApplyDraft(args[0], role, amount, date, reference)
				case "refund":
					draft, text, err = getBooks().ServiceRefundDraft(transaction, args[0], role)
				}
				if err != nil {
					return err
				}
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), text); err != nil {
					return err
				}
				if !save {
					_, err := fmt.Fprintln(cmd.OutOrStdout(), "Preview only. Review accounting/policy and use --save. Reconfirm billing after saved activity.")
					return err
				}
				backup, err := draft.Save()
				if err != nil {
					return err
				}
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "Saved %s. Reconfirm billing after reviewing this activity. Recovery copy: %s\n", action, backup)
				return err
			},
		}
		cmd.Flags().BoolVar(&save, "save", false, "save this reviewed journal step")
		if action == "refund" {
			cmd.Flags().StringVar(&transaction, "transaction", "", "existing actual bank disbursement ID (revised, not duplicated)")
			mustRequireFlags(cmd, "transaction")
		} else {
			cmd.Flags().StringVar(&date, "date", "", "journal posting date, YYYY-MM-DD")
			cmd.Flags().StringVar(&amount, "amount", "", "exact USD amount; adjust accepts negative amounts for credits")
			cmd.Flags().StringVar(&reference, "reference", "", "required operator-approved policy/evidence reference")
			mustRequireFlags(cmd, "date", "amount", "reference")
		}
		if action == "adjust" {
			cmd.Flags().StringVar(&through, "through", "", "inclusive end of service actually covered by this charge/credit")
			cmd.Flags().StringVar(&account, "account", "", "explicit approved nonbank counteraccount; no customer transfers")
			mustRequireFlags(cmd, "through", "account")
		} else {
			cmd.Flags().StringVar(&role, "role", "", "deposit or credit; refund also permits a receivable credit")
			mustRequireFlags(cmd, "role")
		}
		commands = append(commands, cmd)
	}
	return commands
}
