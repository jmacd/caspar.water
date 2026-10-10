package ledger

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

const (
	testUsers = `Account Name,User Name,Service Address,Billing Address,First Period Start,Commercial
School,School,"1 Road; Caspar, CA 91234","1 Road; Caspar, CA 91234",10/1/1914,TRUE
House2,Miller,"2 Road; Caspar, CA 91234","2 Road; Caspar, CA 91234",10/1/1914,FALSE
House3,Sawyer,"3 Road; Caspar, CA 91234","3 Road; Caspar, CA 91234",10/1/1914,FALSE
House4,Vacant,"4 Road; Caspar, CA 91234","4 Road; Caspar, CA 91234",10/1/1914,FALSE
`
	testBusiness = `Name,Address,Contact
"Water Company","1 Drive; Caspar, CA 91234",test@water.com
`
	testCycles = `Period Start,Operations,Utilities,Insurance,Taxes,Bill Date,Method,Margin,Effective Connections,Inactive
10/1/1914,"$300.00","$300.00","$600.00","$600.00",5/1/1915,Introductory,0.0,3,House4
4/1/1915,"$300.00","$300.00","$0.00","$0.00",10/15/1915,Introductory,0.0,3,House4
10/1/1915,"$300.00","$300.00","$600.00","$600.00",4/16/1916,Normal,0.0,4,House4
4/1/1916,"$300.00","$300.00","$0.00","$0.00",10/10/1916,Normal,0.1,4,House4
`
	testPayments = `Date,Account Name,Amount
6/1/1915,School,$400.00
6/1/1915,House2,$400.00
6/1/1915,House3,$200.00
12/1/1915,School,$400.00
12/1/1915,House2,$400.00
12/1/1915,House3,$600.00
5/1/1916,School,$600.00
5/1/1916,House2,$300.00
5/1/1916,House3,$300.00
`
)

func TestGenerateHistoricalJournals(t *testing.T) {
	fs := fixtureFS(t, testUsers, testCycles, testPayments)
	inputs := fixtureInputs()

	first, err := Generate(inputs, fs)
	require.NoError(t, err)
	second, err := Generate(inputs, fs)
	require.NoError(t, err)
	require.Equal(t, first, second, "identical inputs must produce byte-identical output")

	require.Equal(t, []CycleSummary{
		{Name: "1915-03", Expenses: 120000, Assessments: 120000, Payments: 0, Unassigned: 0},
		{Name: "1915-09", Expenses: 120000, Assessments: 120000, Payments: 100000, Unassigned: 0},
		{Name: "1916-03", Expenses: 120000, Assessments: 120000, Payments: 140000, Unassigned: 0},
		{Name: "1916-09", Expenses: 120000, Assessments: 132000, Payments: 120000, Unassigned: 0},
	}, first.Cycles)
	require.Contains(t, string(first.Files["accounts.journal"]), "Assets:Accounts Receivable:School")
	require.Contains(t, string(first.Files["cycles/1916-03.journal"]), "Income:Water Service")
	require.Contains(t, string(first.Files["cycles/1915-09.journal"]), "payments.csv:2")

	root := t.TempDir()
	output := filepath.Join(root, "ledger-history")
	require.NoError(t, Write(output, first, afero.NewOsFs()))
	require.ErrorContains(t, Write(output, first, afero.NewOsFs()), "already exists")

	hledger, err := exec.LookPath("hledger")
	if err == nil {
		cmd := exec.Command(
			hledger,
			"-f", filepath.Join(output, "books.journal"),
			"--strict",
			"check",
		)
		combined, checkErr := cmd.CombinedOutput()
		require.NoError(t, checkErr, "hledger check failed:\n%s", combined)
	}
}

func TestGenerateRejectsInvalidSourceRelationships(t *testing.T) {
	tests := []struct {
		name     string
		users    string
		cycles   string
		payments string
		want     string
	}{
		{
			name:     "duplicate customer",
			users:    testUsers + "School,Duplicate,\"5 Road\",\"5 Road\",10/1/1914,FALSE\n",
			cycles:   testCycles,
			payments: testPayments,
			want:     "duplicate customer account name",
		},
		{
			name:   "payment outside cycles",
			users:  testUsers,
			cycles: testCycles,
			payments: testPayments +
				"10/1/1916,School,$1.00\n",
			want: "outside historical cycle coverage",
		},
		{
			name:   "duplicate payment",
			users:  testUsers,
			cycles: testCycles,
			payments: testPayments +
				"6/1/1915,School,$400.00\n",
			want: "duplicate payment",
		},
		{
			name:     "cycle gap",
			users:    testUsers,
			cycles:   testCycles[:len(testCycles)-len("4/1/1916,\"$300.00\",\"$300.00\",\"$0.00\",\"$0.00\",10/10/1916,Normal,0.1,4,House4\n")] + "10/1/1916,\"$300.00\",\"$300.00\",\"$0.00\",\"$0.00\",4/10/1917,Normal,0.1,4,House4\n",
			payments: testPayments,
			want:     "expected contiguous start",
		},
		{
			name:     "zero effective connections",
			users:    testUsers,
			cycles:   strings.Replace(testCycles, "0.0,3,House4", "0.0,0,House4", 1),
			payments: testPayments,
			want:     "effective connection count must be positive",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fs := fixtureFS(t, test.users, test.cycles, test.payments)
			_, err := Generate(fixtureInputs(), fs)
			require.ErrorContains(t, err, test.want)
		})
	}
}

func TestGenerateReportsHistoricallyUnassignedShares(t *testing.T) {
	cycles := strings.Replace(
		testCycles,
		"5/1/1915,Introductory,0.0,3,House4",
		"5/1/1915,Introductory,0.0,4,House4",
		1,
	)
	result, err := Generate(
		fixtureInputs(),
		fixtureFS(t, testUsers, cycles, testPayments),
	)
	require.NoError(t, err)
	require.Equal(t, int64(30000), result.Cycles[0].Unassigned)
	require.Equal(t, int64(90000), result.Cycles[0].Assessments)
	require.Contains(
		t,
		string(result.Files["cycles/1915-03.journal"]),
		"Historical unbilled effective-connection shares: $300.00",
	)
}

func fixtureInputs() Inputs {
	return Inputs{
		UsersFile:    "users.csv",
		BusinessFile: "business.csv",
		CyclesFile:   "cycles.csv",
		PaymentsFile: "payments.csv",
	}
}

func fixtureFS(t *testing.T, users, cycles, payments string) afero.Fs {
	t.Helper()
	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "users.csv", []byte(users), 0o644))
	require.NoError(t, afero.WriteFile(fs, "business.csv", []byte(testBusiness), 0o644))
	require.NoError(t, afero.WriteFile(fs, "cycles.csv", []byte(cycles), 0o644))
	require.NoError(t, afero.WriteFile(fs, "payments.csv", []byte(payments), 0o644))
	return fs
}
