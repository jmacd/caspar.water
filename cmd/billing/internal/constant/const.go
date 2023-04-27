package constant

const (
	CsvLayout         = "1/2/2006"
	InvoiceDateLayout = "2006-Jan"
	FullDateLayout    = "January 2, 2006"

	// MaxConnections is how many connections we can reach,
	// excluding the one that is not viable (so that with that
	// connection maxConnections would be 14).  Limit is 14.
	MaxConnections = 13

	// CommunityCenterAdjustedUserCount is the target effective
	// user count for the CC used for billing after the initial
	// adjustment, which gives the CC double weight.
	CommunityCenterAdjustedUserCount = 2

	// CommunityCenterAdjustment is how many effective
	// users will be added after the CC adjustment is applied.
	CommunityCenterAdjustment = (CommunityCenterAdjustedUserCount - 1)

	// CommunityCenterAccount is the account name for the
	// community center used to carry out its adjustment.
	CommunityCenterAccount = "Comm_Ctr"

	InitialMargin       = 0.0
	TargetMargin        = 0.2
	MarginIncreaseYears = 2
	StatementsPerYear   = 2

	FirstCycleMonth  = 4
	SecondCycleMonth = 10
)
