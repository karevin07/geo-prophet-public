package domain

// ImportRun describes one execution of a data import for audit in import_runs.
type ImportRun struct {
	Source   string // e.g. "lvr" or "addresspoint"
	Dataset  string // e.g. "sales" or "door_numbers"
	Season   string // 實價登錄 season, or the snapshot month for address points
	CityCode string
}

// ImportStats counts what happened to the rows of an import run.
type ImportStats struct {
	RowsRead          int
	RowsSkipped       int
	RowsUpserted      int
	RowsGeocodeFailed int
	Detail            map[string]int
}
