package main

import (
	"fmt"
	"time"

	"github.com/jmacd/caspar.water/cmd/internal/wat"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func customerCommands(getBooks func() *wat.Books, getConfig func() wat.Config) []*cobra.Command {
	customer := &cobra.Command{
		Use: "customer", Short: "Manage customers in the private file-backed registry",
		RunE: func(_ *cobra.Command, _ []string) error { return wat.RunCustomersTerminal(getBooks()) },
		Args: cobra.NoArgs,
	}
	customer.AddCommand(&cobra.Command{
		Use: "init", Short: "Create registry.yaml without changing configuration or journals",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := wat.InitCustomerRegistry(getConfig()); err != nil {
				return err
			}
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "Created registry.yaml. Set customer_registry: true in wat.yaml, then restart Wat.")
			return err
		},
	})
	connection := &cobra.Command{
		Use: "connection", Short: "Manage physical service connections",
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error { return wat.RunCustomersTerminal(getBooks()) },
	}
	service := &cobra.Command{
		Use: "service", Short: "Open service accounts, allocate recorded receipts, and report",
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error { return wat.RunCustomersTerminal(getBooks()) },
	}
	for _, root := range []*cobra.Command{customer, connection, service} {
		root.AddCommand(&cobra.Command{
			Use: "list", Short: "List registry records (including historical records)", Args: cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				registry, err := getBooks().Customers()
				if err != nil {
					return err
				}
				var records any
				switch root.Name() {
				case "customer":
					records = registry.Customers
				case "connection":
					records = registry.Connections
				case "service":
					records = registry.ServiceAccounts
				}
				return writeCustomerYAML(cmd, records)
			},
		})
	}
	for _, root := range []*cobra.Command{customer, connection} {
		for _, action := range []string{"add", "edit"} {
			var name string
			var address []string
			cmd := &cobra.Command{
				Use: action, Short: action + " a " + root.Name() + " (saves registry.yaml only)",
				Args: cobra.NoArgs,
				RunE: func(cmd *cobra.Command, args []string) error {
					draft, err := getBooks().NewRegistryDraft()
					if err != nil {
						return err
					}
					var id string
					if root == customer {
						if action == "add" {
							id, err = draft.AddCustomer(name, address)
						} else {
							record, lookupErr := draft.Registry.Customer(args[0])
							if lookupErr != nil {
								return lookupErr
							}
							id = record.ID
							for i := range draft.Registry.Customers {
								if draft.Registry.Customers[i].ID == id {
									if cmd.Flags().Changed("payee") {
										draft.Registry.Customers[i].Payee = name
									}
									if cmd.Flags().Changed("address") {
										draft.Registry.Customers[i].BillingAddress.Lines = address
									}
								}
							}
						}
					} else if action == "add" {
						id, err = draft.AddConnection(name, address)
					} else {
						record, lookupErr := draft.Registry.Connection(args[0])
						if lookupErr != nil {
							return lookupErr
						}
						id = record.ID
						for i := range draft.Registry.Connections {
							if draft.Registry.Connections[i].ID == id {
								if cmd.Flags().Changed("name") {
									draft.Registry.Connections[i].Name = name
								}
								if cmd.Flags().Changed("address") {
									draft.Registry.Connections[i].ServiceAddress.Lines = address
								}
							}
						}
					}
					if err != nil {
						return err
					}
					backup, err := draft.Save()
					if err != nil {
						return err
					}
					_, err = fmt.Fprintf(cmd.OutOrStdout(), "Saved %s. Recovery copy: %s\n", id, backup)
					return err
				},
			}
			nameFlag := "name"
			nameHelp := "connection name"
			if root == customer {
				nameFlag = "payee"
				nameHelp = "customer name (declared automatically from the registry)"
			}
			cmd.Flags().StringVar(&name, nameFlag, "", nameHelp)
			cmd.Flags().StringArrayVar(&address, "address", nil, "address line (repeat for additional lines)")
			if action == "edit" {
				cmd.Use += " ID"
				cmd.Args = cobra.ExactArgs(1)
			} else {
				mustRequireFlags(cmd, nameFlag, "address")
			}
			root.AddCommand(cmd)
		}
	}
	var customerSelector, connectionSelector, from, receivable, credit, deposit string
	open := &cobra.Command{
		Use: "open", Short: "Open dated service responsibility without recording money", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			draft, err := getBooks().NewRegistryDraft()
			if err != nil {
				return err
			}
			id, err := draft.OpenService(wat.ServiceAccount{
				Customer: customerSelector, Connection: connectionSelector, ServiceFrom: from,
				ReceivableAccount: receivable, CreditAccount: credit, DepositAccount: deposit,
			})
			if err != nil {
				return err
			}
			backup, err := draft.Save()
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Opened %s. No financial posting. Recovery copy: %s\n", id, backup)
			return err
		},
	}
	open.Flags().StringVar(&customerSelector, "customer", "", "customer ID or exact unambiguous payee")
	open.Flags().StringVar(&connectionSelector, "connection", "", "connection ID or exact unambiguous name")
	open.Flags().StringVar(&from, "from", "", "first service date, YYYY-MM-DD")
	open.Flags().StringVar(&receivable, "receivable", "", "reviewed 141 account; defaults to Asset:141.000:Customers:SA-ID")
	open.Flags().StringVar(&credit, "credit", "", "optional explicitly approved non-deposit liability account")
	open.Flags().StringVar(&deposit, "deposit", "", "optional explicitly approved 233 security-deposit account")
	mustRequireFlags(open, "customer", "connection", "from")
	service.AddCommand(open)
	service.AddCommand(serviceLifecycleCommands(getBooks)...)

	for _, root := range []*cobra.Command{customer, connection, service} {
		var asOf, reportFrom string
		var issue bool
		show := &cobra.Command{
			Use: "show ID", Short: "Show identity, service history, and separate ledger balances",
			Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				if issue {
					if reportFrom != "" {
						return fmt.Errorf("issued closing statements use the complete service history; omit --from")
					}
					path, err := getBooks().IssueClosingStatement(args[0], asOf)
					if err != nil {
						return err
					}
					_, err = fmt.Fprintln(cmd.OutOrStdout(), "Issued "+path+". No financial posting.")
					return err
				}
				text, err := getBooks().CustomerReport(root.Name(), args[0], reportFrom, asOf)
				if err != nil {
					return err
				}
				_, err = fmt.Fprint(cmd.OutOrStdout(), text)
				return err
			},
		}
		if root == service {
			show.Aliases = []string{"statement"}
			show.Flags().BoolVar(&issue, "issue", false, "issue a new immutable closing statement after billing confirmation")
		}
		show.Flags().StringVar(&asOf, "as-of", time.Now().Format("2006-01-02"), "inclusive report end date")
		show.Flags().StringVar(&reportFrom, "from", "", "report start date; defaults to each service start")
		root.AddCommand(show)
	}
	var allocations []string
	var save bool
	allocate := &cobra.Command{
		Use: "allocate TRANSACTION-ID", Short: "Preview allocating an existing bank receipt; --save revises it",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			draft, text, err := getBooks().ServicePaymentDraft(args[0], allocations)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), text); err != nil {
				return err
			}
			if !save {
				_, err := fmt.Fprintln(cmd.OutOrStdout(), "Preview only. Review classification and use --save to revise this receipt; no new cash entry.")
				return err
			}
			backup, err := draft.Save()
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Revised existing receipt. Recovery copy: %s\n", backup)
			return err
		},
	}
	allocate.Flags().StringArrayVar(&allocations, "to", nil, "SA-ID[:receivable|credit|deposit]=AMOUNT; repeat for split receipts")
	allocate.Flags().BoolVar(&save, "save", false, "save the reviewed allocation to the original journal entry")
	mustRequireFlags(allocate, "to")
	service.AddCommand(allocate)

	payee := &cobra.Command{Use: "payee", Short: "Manage native hledger payee declarations"}
	payee.AddCommand(&cobra.Command{
		Use: "init", Short: "Import manual payee declarations into payees.yaml without overwriting existing metadata",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			backup, err := wat.InitPayeeRegistry(getConfig())
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "Created payees.yaml. Set payee_registry: true in wat.yaml. Original declarations: "+backup)
			return err
		},
	}, &cobra.Command{
		Use: "list", Short: "List canonical payees and aliases from the YAML registry",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			registry, err := getBooks().Payees()
			if err != nil {
				return err
			}
			return writeCustomerYAML(cmd, registry)
		},
	}, &cobra.Command{
		Use: "alias NAME ALIAS", Short: "Add an explicit alias to a canonical payee",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			backup, _, err := getBooks().AddPayeeAlias(args[0], args[1])
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "Saved alias. Recovery copy: "+backup)
			return err
		},
	})
	payee.AddCommand(&cobra.Command{
		Use: "add NAME", Short: "Add a canonical payee (or a manual declaration when the registry is disabled)", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			backup, _, err := getBooks().DeclarePayee(args[0])
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Declared payee %q. Recovery copy: %s\n", args[0], backup)
			return err
		},
	})
	return []*cobra.Command{customer, connection, service, payee}
}

func mustRequireFlags(cmd *cobra.Command, names ...string) {
	for _, name := range names {
		if err := cmd.MarkFlagRequired(name); err != nil {
			panic(err)
		}
	}
}

func writeCustomerYAML(cmd *cobra.Command, value any) error {
	data, err := yaml.Marshal(value)
	if err != nil {
		return err
	}
	_, err = cmd.OutOrStdout().Write(data)
	return err
}
