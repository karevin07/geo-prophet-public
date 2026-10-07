package domain

// InvestmentMonthlyCount is an observed count of qualifying registered sales.
type InvestmentMonthlyCount struct {
	District         string `json:"-"`
	Month            string `json:"month"`
	Count            int    `json:"count"`
	CoverageComplete bool   `json:"coverageComplete"`
}

type InvestmentLiquidityArea struct {
	District                   string                   `json:"district"`
	CurrentCount               int                      `json:"currentCount"`
	PreviousCount              int                      `json:"previousCount"`
	ActiveMonths               *int                     `json:"activeMonths"`
	LongestZeroRun             *int                     `json:"longestZeroRun"`
	VolumeChange               *float64                 `json:"volumeChange"`
	SufficientCurrentSample    bool                     `json:"sufficientCurrentSample"`
	SufficientComparisonSample bool                     `json:"sufficientComparisonSample"`
	Monthly                    []InvestmentMonthlyCount `json:"monthly"`
}

type InvestmentLiquidityReport struct {
	CityCode                 string                    `json:"cityCode"`
	Through                  string                    `json:"through"`
	CurrentFrom              string                    `json:"currentFrom"`
	PreviousFrom             string                    `json:"previousFrom"`
	MinimumSample            int                       `json:"minimumSample"`
	CurrentCoverageComplete  bool                      `json:"currentCoverageComplete"`
	PreviousCoverageComplete bool                      `json:"previousCoverageComplete"`
	IncompleteMonths         []string                  `json:"incompleteMonths"`
	Areas                    []InvestmentLiquidityArea `json:"areas"`
}
