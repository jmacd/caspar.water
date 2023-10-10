package logic

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestLogic(t *testing.T) {

	fs := afero.NewMemMapFs()
	afs := &afero.Afero{Fs: fs}

	require.NoError(t, afs.WriteFile("users.csv", []byte(`
Account Name,User Name,Service Address,Billing Address,Active,First Period Start
Comm_Ctr,School,"1 Road; Caspar, CA 91234","1 Road; Caspar, CA 91234",TRUE,10/1/1914
House1,Miller,"2 Road; Caspar, CA 91234","2 Road; Caspar, CA 91234",TRUE,10/1/1914
House2,Sawyer,"3 Road; Caspar, CA 91234","3 Road; Caspar, CA 91234",TRUE,10/1/1914
`), 0644))

	require.NoError(t, afs.WriteFile("business.csv", []byte(`
Name,Address,Contact
"Water Company","1 Drive; Caspar, CA 91234",p: 555-555-5555; e: test@water.com
`), 0644))

	require.NoError(t, afs.WriteFile("cycles.csv", []byte(`
Period Start,Operations,Utilities,Insurance,Taxes,Bill Date,Method
10/1/1914,"$300.00",$300.00,"$600.00","$600.00",5/1/1915,Baseline
4/1/1915,"$300.00","$300.00","$0.00","$0.00",10/15/1915,Baseline
10/1/1915,"$300.00",$300.00,"$600.00","$600.00",4/16/1916,FirstAdjustment
4/1/1916,"$300.00","$300.00","$0.00","$0.00",10/10/1916,NormalAdjustment
`), 0644))

	require.NoError(t, afs.WriteFile("payments.csv", []byte(`
Date,Account Name,Amount
6/1/1915,Comm_Ctr,$400.00
6/1/1915,House1,$400.00
6/1/1915,House2,$200.00
12/1/1915,Comm_Ctr,$400.00
12/1/1915,House1,$400.00
12/1/1915,House2,$600.00
`), 0644))

	require.NoError(t, afs.Mkdir("stmts", 0644))
	require.NoError(t, afs.WriteFile("stmts/1915-Mar.txt", []byte("hello world\n"), 0644))
	require.NoError(t, afs.WriteFile("stmts/1915-Sep.txt", []byte("hello world\n"), 0644))
	require.NoError(t, afs.WriteFile("stmts/1916-Mar.txt", []byte("hello world\n"), 0644))
	require.NoError(t, afs.WriteFile("stmts/1916-Sep.txt", []byte("hello world\n"), 0644))

	result, err := Logic(Inputs{
		InitialConnectionCount: 3,
		UsersFile:              "users.csv",
		BusinessFile:           "business.csv",
		CyclesFile:             "cycles.csv",
		PaymentsFile:           "payments.csv",
		StatementsDir:          "stmts",
	}, fs)
	require.NoError(t, err)

	require.Equal(t, 4, len(result.Cycles))

	cycle0 := result.Cycles[0]
	require.Equal(t, 3, len(cycle0.Statements))

	require.Equal(t, cycle0.Statements[0].User.UserName, "School")
	require.Equal(t, cycle0.Statements[1].User.UserName, "Miller")
	require.Equal(t, cycle0.Statements[2].User.UserName, "Sawyer")

	require.Equal(t, cycle0.Statements[0].Vars.TotalDue, "$400.00")
	require.Equal(t, cycle0.Statements[1].Vars.TotalDue, "$400.00")
	require.Equal(t, cycle0.Statements[2].Vars.TotalDue, "$400.00")

	cycle1 := result.Cycles[1]
	require.Equal(t, 3, len(cycle1.Statements))
	require.Equal(t, cycle1.Statements[0].Vars.TotalDue, "$400.00")
	require.Equal(t, cycle1.Statements[1].Vars.TotalDue, "$400.00")
	require.Equal(t, cycle1.Statements[2].Vars.TotalDue, "$600.00")

	cycle2 := result.Cycles[2]
	require.Equal(t, 3, len(cycle2.Statements))
	require.Equal(t, cycle2.Statements[0].Vars.TotalDue, "$600.00")
	require.Equal(t, cycle2.Statements[1].Vars.TotalDue, "$300.00")
	require.Equal(t, cycle2.Statements[2].Vars.TotalDue, "$300.00")

	cycle3 := result.Cycles[3]
	require.Equal(t, 3, len(cycle3.Statements))
	require.Equal(t, cycle3.Statements[0].Vars.TotalDue, "$1,230.00")
	require.Equal(t, cycle3.Statements[1].Vars.TotalDue, "$615.00")
	require.Equal(t, cycle3.Statements[2].Vars.TotalDue, "$615.00")
}
