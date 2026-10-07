package importer

import "time"

// SetClock lets tests pin the time used for season and freshness decisions.
func SetClock(service *Service, now func() time.Time) { service.now = now }
