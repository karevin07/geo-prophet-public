package domain

import "time"

// FeedSort orders the transaction feed; every order breaks ties by id for stable paging.
type FeedSort string

const (
	SortDateDesc         FeedSort = "date_desc"
	SortDateAsc          FeedSort = "date_asc"
	SortPricePerPingDesc FeedSort = "price_per_ping_desc"
	SortPricePerPingAsc  FeedSort = "price_per_ping_asc"
	SortTotalPriceDesc   FeedSort = "total_price_desc"
)

var FeedSorts = []FeedSort{SortDateDesc, SortDateAsc, SortPricePerPingDesc, SortPricePerPingAsc, SortTotalPriceDesc}

// FeedCursor is the sort value and id of the last row on the previous page.
type FeedCursor struct {
	Value string `json:"v"`
	ID    string `json:"id"`
}

// FeedQuery pages through transactions matching the shared filter plus a region.
type FeedQuery struct {
	Filter            TransactionFilter
	CityCode          string
	District          string
	LatestReleaseOnly bool
	Sort              FeedSort
	Cursor            *FeedCursor
}

// Release identifies a 10-day publication, e.g. 1150901-1150910.
type Release struct {
	Label          string    `json:"label"`
	RegisteredFrom time.Time `json:"registeredFrom"`
	RegisteredTo   time.Time `json:"registeredTo"`
}

type FeedPage struct {
	Transactions []Transaction `json:"data"`
	// Total and NewCount are only computed for the first page.
	Total         *int        `json:"total,omitempty"`
	NewCount      *int        `json:"newCount,omitempty"`
	NextCursor    *FeedCursor `json:"-"`
	LatestRelease *Release    `json:"latestRelease"`
}

// Region is a city with its districts, for scoping the feed.
type Region struct {
	Code      string   `json:"code"`
	Name      string   `json:"name"`
	Districts []string `json:"districts"`
}
