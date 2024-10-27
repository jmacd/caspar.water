# Invoice generator

This tool is meant for irregularly-generated invoices, as opposed to
the [billing program for recurring payments](../billing/README.md).

## Example

Create an invoice input using the YAML syntax shown below:

```yaml
job_name: Emergency repair
account: 10AddressSt
date: 2021-01-15
items:
- desc: Shovel @ $1/hr
  amount: $6.00
- desc: Backhoe @ $2.50/hr
  amount: $2.50
- desc: Plumbing @ $2/hr
  amount: $6.00
- desc: Materials
  amount: $5.00
```

Run the program:

```
go run ./cmd/invoice --input inv01.yaml --output inv01.pdf
```
