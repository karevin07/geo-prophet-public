package domain

import (
	"encoding/json"
	"errors"
	"time"
)

// Identity is the Tailscale user behind a request, from the headers `tailscale serve`
// adds; Login is empty when the request did not come through it.
type Identity struct {
	Login      string `json:"login"`
	Name       string `json:"name"`
	ProfilePic string `json:"profilePic"`
}

// AdminSession tells the admin console who is signed in and whether they may use it.
type AdminSession struct {
	Identity
	Admin bool `json:"admin"`
}

type TableSize struct {
	Name  string `json:"name"`
	Bytes int64  `json:"bytes"`
}

type DatabaseUsage struct {
	Bytes        int64       `json:"bytes"`
	Transactions int64       `json:"transactions"`
	Tables       []TableSize `json:"tables"`
}

// DiskUsage is the filesystem holding the backups, which on this deployment is the host's.
type DiskUsage struct {
	TotalBytes uint64 `json:"totalBytes"`
	UsedBytes  uint64 `json:"usedBytes"`
	FreeBytes  uint64 `json:"freeBytes"`
}

type BackupFile struct {
	Name       string    `json:"name"`
	Bytes      int64     `json:"bytes"`
	ModifiedAt time.Time `json:"modifiedAt"`
}

type SystemStatus struct {
	GeneratedAt time.Time     `json:"generatedAt"`
	Database    DatabaseUsage `json:"database"`
	// Disk and Backups are nil when the backup directory is not mounted.
	Disk    *DiskUsage   `json:"disk"`
	Backups []BackupFile `json:"backups"`
	// BackupKeep and BackupIntervalHours mirror the backup service's settings.
	BackupKeep          int `json:"backupKeep"`
	BackupIntervalHours int `json:"backupIntervalHours"`
}

// JobKind is work the admin console can request.
type JobKind string

const (
	JobSync           JobKind = "sync"            // the importer's full sync
	JobCurrentRelease JobKind = "current_release" // re-import the latest 10-day release
	JobTransit        JobKind = "transit"         // reload rail stations from TDX
	JobBackup         JobKind = "backup"          // run by the backup service
	JobSeasons        JobKind = "seasons"         // re-import chosen seasons and cities
)

// ErrJobActive means a job of the same kind is already queued or running.
var ErrJobActive = errors.New("a job of this kind is already queued or running")

// JobKinds lists every kind in display order.
var JobKinds = []JobKind{JobSync, JobCurrentRelease, JobTransit, JobBackup, JobSeasons}

type JobStatus string

const (
	JobQueued    JobStatus = "queued"
	JobRunning   JobStatus = "running"
	JobSucceeded JobStatus = "succeeded"
	JobFailed    JobStatus = "failed"
)

// AdminJob is one requested run, kept as an audit record of who asked for what.
type AdminJob struct {
	ID          int64     `json:"id"`
	Kind        JobKind   `json:"kind"`
	Status      JobStatus `json:"status"`
	RequestedBy string    `json:"requestedBy"`
	RequestedAt time.Time `json:"requestedAt"`
	// Params is the job's input, e.g. {"seasons": "115S2", "cities": ["a"]} for seasons.
	Params     json.RawMessage `json:"params"`
	StartedAt  *time.Time      `json:"startedAt"`
	FinishedAt *time.Time      `json:"finishedAt"`
	// Result summarizes the work: {"runs", "rowsUpserted", "failedRuns"} for imports and
	// {"file", "bytes"} for backups.
	Result json.RawMessage `json:"result"`
	Error  string          `json:"error,omitempty"`
}

// ImportSummary is an import job's result: the import runs it made.
type ImportSummary struct {
	Runs         int `json:"runs"`
	FailedRuns   int `json:"failedRuns"`
	RowsUpserted int `json:"rowsUpserted"`
}

// LineOpening is a line and its opening date; OpenedOn is nil when it opened before 2015.
type LineOpening struct {
	LineID    string     `json:"lineId"`
	Name      string     `json:"name"`
	System    string     `json:"system"`
	Color     string     `json:"color"`
	OpenedOn  *string    `json:"openedOn"` // YYYY-MM-DD
	Source    string     `json:"source"`
	UpdatedBy string     `json:"updatedBy"`
	UpdatedAt *time.Time `json:"updatedAt"`
}

// PlatformOpening is a platform that opened after its line, e.g. an extension.
type PlatformOpening struct {
	PlatformID   string    `json:"platformId"`
	PlatformName string    `json:"platformName"`
	StationName  string    `json:"stationName"`
	Lines        string    `json:"lines"` // names of the lines it serves
	LineColor    string    `json:"lineColor"`
	LineOpenedOn *string   `json:"lineOpenedOn"`
	OpenedOn     string    `json:"openedOn"`
	Source       string    `json:"source"`
	UpdatedBy    string    `json:"updatedBy"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// PlatformOption is a platform that can be given its own opening date.
type PlatformOption struct {
	PlatformID   string `json:"platformId"`
	PlatformName string `json:"platformName"`
	StationName  string `json:"stationName"`
	Lines        string `json:"lines"`
}

type Openings struct {
	Lines     []LineOpening     `json:"lines"`
	Platforms []PlatformOpening `json:"platforms"`
	Options   []PlatformOption  `json:"options"`
}

// OpeningEdit sets or, with a nil OpenedOn, removes the opening date of a line or platform.
type OpeningEdit struct {
	Kind     string  `json:"kind"` // line or platform
	TargetID string  `json:"targetId"`
	OpenedOn *string `json:"openedOn"`
	Source   string  `json:"source"`
}

// ErrUnknownTarget means an opening edit names a line or platform not in the network.
var ErrUnknownTarget = errors.New("no such line or platform")
