package importer_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/geo-prophet/geo-prophet/api/internal/importer"
	"github.com/geo-prophet/geo-prophet/api/internal/transit"
)

type finished struct {
	id     int64
	result any
	err    error
}

type jobStoreStub struct {
	queue       []domain.AdminJob
	claimKinds  []domain.JobKind
	finished    []finished
	interrupted []domain.JobKind
}

func (stub *jobStoreStub) ClaimJob(_ context.Context, kinds []domain.JobKind) (domain.AdminJob, bool, error) {
	stub.claimKinds = kinds
	if len(stub.queue) == 0 {
		return domain.AdminJob{}, false, nil
	}
	job := stub.queue[0]
	stub.queue = stub.queue[1:]
	return job, true, nil
}

func (stub *jobStoreStub) FinishJob(_ context.Context, id int64, result any, jobErr error) error {
	stub.finished = append(stub.finished, finished{id, result, jobErr})
	return nil
}

func (stub *jobStoreStub) FailInterruptedJobs(_ context.Context, kinds []domain.JobKind) (int, error) {
	stub.interrupted = kinds
	return 1, nil
}

func (*jobStoreStub) ImportSummary(context.Context, time.Time) (domain.ImportSummary, error) {
	return domain.ImportSummary{Runs: 1, RowsUpserted: 201}, nil
}

func TestJobsRunQueuedTransitImportsAndRecordOutcomes(t *testing.T) {
	loads := 0
	source := func(context.Context) (transit.Network, error) {
		loads++
		if loads == 2 {
			return transit.Network{}, errors.New("TDX: 429 Too Many Requests")
		}
		return transit.Network{Stations: []transit.Station{{ID: "TRTC-BL07"}}}, nil
	}
	store := &jobStoreStub{queue: []domain.AdminJob{{ID: 1, Kind: domain.JobTransit}, {ID: 2, Kind: domain.JobTransit}}}
	jobs := &importer.Jobs{Service: newServiceWithTransit(newStoreStub(), &fetcherStub{}, source), Store: store, Cities: []string{"a"}}

	if err := jobs.RunQueued(context.Background()); err != nil {
		t.Fatalf("RunQueued: %v", err)
	}
	if len(store.finished) != 2 || store.finished[0].err != nil || store.finished[1].err == nil {
		t.Fatalf("finished = %+v", store.finished)
	}
	if summary, ok := store.finished[0].result.(domain.ImportSummary); !ok || summary.RowsUpserted != 201 {
		t.Fatalf("result = %#v", store.finished[0].result)
	}
	if slices.Contains(store.claimKinds, domain.JobBackup) {
		t.Fatal("the importer must not claim backup jobs")
	}
	if err := jobs.RecoverInterrupted(context.Background()); err != nil || !slices.Equal(store.interrupted, importer.ImporterJobs) {
		t.Fatalf("recovered %v, %v", store.interrupted, err)
	}
}

func TestJobsRunSeasonImports(t *testing.T) {
	store := newStoreStub()
	store.points["a"] = taipeiPoints()
	fetcher := &fetcherStub{files: map[string][]byte{"https://example.test/115S1.zip": archive(t, nil), "https://example.test/115S2.zip": archive(t, nil)}}
	queue := &jobStoreStub{queue: []domain.AdminJob{{ID: 1, Kind: domain.JobSeasons, Params: []byte(`{"seasons":"115S1-115S2","cities":["a"]}`)}}}
	jobs := &importer.Jobs{Service: newService(store, fetcher), Store: queue, Cities: []string{"a", "f", "h"}}
	if err := jobs.RunQueued(context.Background()); err != nil {
		t.Fatal(err)
	}
	if queue.finished[0].err != nil {
		t.Fatalf("job failed: %v", queue.finished[0].err)
	}
	// Only the requested city, not every city the importer syncs.
	if got := store.runsFor("sales"); !slices.Equal(got, []string{"115S1/a", "115S2/a"}) {
		t.Fatalf("imported %v", got)
	}
}
