package usecase

import (
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/lvr"
)

// registrationLag is how long after a sale its registration may still be published:
// transfer registration takes weeks and declaration is due 30 days after it.
const registrationLag = 90 * 24 * time.Hour

// registeredThrough returns the last registration date up to which the imported season
// archives and 10-day releases are continuous. It starts at the newest season and follows
// releases (or later seasons) that begin the day after the covered range ends.
func registeredThrough(seasons, releases []string) (time.Time, bool) {
	type window struct{ from, to time.Time }
	var windows []window
	var through time.Time
	for _, label := range seasons {
		season, err := lvr.ParseSeason(label)
		if err != nil {
			continue
		}
		from, to := season.RegistrationWindow()
		windows = append(windows, window{from, to})
		if to.After(through) {
			through = to
		}
	}
	if through.IsZero() {
		return time.Time{}, false
	}
	for _, label := range releases {
		if release, err := lvr.ParseReleaseLabel(label); err == nil {
			windows = append(windows, window{release.RegisteredFrom, release.RegisteredTo})
		}
	}
	for extended := true; extended; {
		extended = false
		for _, candidate := range windows {
			if !candidate.from.After(through.AddDate(0, 0, 1)) && candidate.to.After(through) {
				through, extended = candidate.to, true
			}
		}
	}
	return through, true
}

// completeThrough returns the first day of the last month whose sales are all registered,
// given registrations are covered through the given date.
func completeThrough(registered time.Time) time.Time {
	cutoff := registered.Add(-registrationLag)
	month := time.Date(cutoff.Year(), cutoff.Month(), 1, 0, 0, 0, 0, time.UTC)
	if cutoff.AddDate(0, 0, 1).Month() == cutoff.Month() {
		month = month.AddDate(0, -1, 0) // the cutoff's own month is not over yet
	}
	return month
}
