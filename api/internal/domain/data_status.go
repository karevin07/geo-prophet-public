package domain

import "time"

// ImportRunStatus is a public, credential-free summary of one importer attempt.
type ImportRunStatus struct {
	ID                int64      `json:"id"`
	Source            string     `json:"source"`
	Dataset           string     `json:"dataset"`
	Season            string     `json:"season"`
	CityCode          string     `json:"cityCode"`
	Status            string     `json:"status"`
	RowsRead          int        `json:"rowsRead"`
	RowsSkipped       int        `json:"rowsSkipped"`
	RowsUpserted      int        `json:"rowsUpserted"`
	RowsGeocodeFailed int        `json:"rowsGeocodeFailed"`
	StartedAt         time.Time  `json:"startedAt"`
	FinishedAt        *time.Time `json:"finishedAt"`
	// ErrorKind classifies a failure (see usecase.ErrorKind). Error is the full message,
	// which may hold URLs or internals; the status is only served to admins.
	ErrorKind string `json:"errorKind,omitempty"`
	Error     string `json:"error,omitempty"`
}

type CityDataCount struct {
	CityCode       string     `json:"cityCode"`
	Transactions   int64      `json:"transactions"`
	Located        int64      `json:"located"`
	FutureDated    int64      `json:"futureDated"`
	LatestSaleDate *time.Time `json:"latestSaleDate"`
	// Leases (租賃); LatestLeaseDate ignores lease dates in the future.
	Rentals         int64      `json:"rentals"`
	RentalsLocated  int64      `json:"rentalsLocated"`
	LatestLeaseDate *time.Time `json:"latestLeaseDate"`
}

type SeasonCoverage struct {
	CityCode       string   `json:"cityCode"`
	FirstImported  string   `json:"firstImported"`
	LatestImported string   `json:"latestImported"`
	Missing        []string `json:"missing"`
}

type DataStatusSnapshot struct {
	RecentRuns         []ImportRunStatus   `json:"recentRuns"`
	LatestRuns         []ImportRunStatus   `json:"latestRuns"`
	LastSuccessfulRuns []ImportRunStatus   `json:"lastSuccessfulRuns"`
	Cities             []CityDataCount     `json:"cities"`
	ImportedSeasons    map[string][]string `json:"-"`
	QualityRuns        []QualityRun        `json:"-"`
}

// QualityRun contains the counters needed to assess one successful sales import.
// RowsUpserted includes updates and must never be described as new sales.
type QualityRun struct {
	ID                int64          `json:"id"`
	Dataset           string         `json:"dataset"`
	Season            string         `json:"season"`
	CityCode          string         `json:"cityCode"`
	RowsRead          int            `json:"rowsRead"`
	RowsSkipped       int            `json:"rowsSkipped"`
	RowsUpserted      int            `json:"rowsUpserted"`
	RowsGeocodeFailed int            `json:"rowsGeocodeFailed"`
	Detail            map[string]int `json:"-"`
	FinishedAt        time.Time      `json:"finishedAt"`
}

type QualityMetric struct {
	CityCode          string    `json:"cityCode"`
	Dataset           string    `json:"dataset"`
	Season            string    `json:"season"`
	RowsRead          int       `json:"rowsRead"`
	RowsSkipped       int       `json:"rowsSkipped"`
	RowsUpserted      int       `json:"rowsUpserted"`
	RowsGeocodeFailed int       `json:"rowsGeocodeFailed"`
	PreciseCount      int       `json:"preciseCount"`
	LocatedCount      int       `json:"locatedCount"`
	PreciseRate       *float64  `json:"preciseRate"`
	LocatedRate       *float64  `json:"locatedRate"`
	PreviousRowsRead  *int      `json:"previousRowsRead"`
	VolumeChange      *float64  `json:"volumeChange"`
	FinishedAt        time.Time `json:"finishedAt"`
}

type QualityAlert struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	CityCode string `json:"cityCode"`
	Dataset  string `json:"dataset"`
	Season   string `json:"season"`
	RunID    int64  `json:"runId"`
}

type QualityStatus struct {
	Metrics []QualityMetric `json:"metrics"`
	Alerts  []QualityAlert  `json:"alerts"`
}

type DataStatus struct {
	GeneratedAt        time.Time         `json:"generatedAt"`
	RecentRuns         []ImportRunStatus `json:"recentRuns"`
	LatestRuns         []ImportRunStatus `json:"latestRuns"`
	LastSuccessfulRuns []ImportRunStatus `json:"lastSuccessfulRuns"`
	Cities             []CityDataCount   `json:"cities"`
	Coverage           []SeasonCoverage  `json:"coverage"`
	Quality            QualityStatus     `json:"quality"`
}
