package usecase

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
)

type qualityTransport func(*http.Request) (*http.Response, error)

func (transport qualityTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

type qualityStoreStub struct {
	snapshot domain.DataStatusSnapshot
	queued   []domain.QualityAlert
	result   error
}

func (store *qualityStoreStub) QualitySnapshot(context.Context) (domain.DataStatusSnapshot, error) {
	return store.snapshot, nil
}

func (store *qualityStoreStub) QueueQualityAlerts(_ context.Context, alerts []domain.QualityAlert) error {
	store.queued = alerts
	return nil
}

func (store *qualityStoreStub) ResolveInactiveQualityAlerts(context.Context, []domain.QualityAlert) error {
	return nil
}

func (store *qualityStoreStub) PendingQualityAlerts(context.Context) ([]domain.QualityAlert, error) {
	return store.queued, nil
}

func (store *qualityStoreStub) FinishQualityAlert(_ context.Context, _ domain.QualityAlert, result error) error {
	store.result = result
	return nil
}

func TestQualityPublisherSendsCurrentAlertWithBearerToken(t *testing.T) {
	now := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	webhook, err := NewQualityWebhook("https://example.test/quality", "secret")
	if err != nil {
		t.Fatal(err)
	}
	called := 0
	webhook.client.Transport = qualityTransport(func(request *http.Request) (*http.Response, error) {
		called++
		if request.Method != http.MethodPost || request.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("incorrect webhook request: %s %s", request.Method, request.Header.Get("Authorization"))
		}
		body, _ := io.ReadAll(request.Body)
		if !strings.Contains(string(body), `"code":"stale_release"`) || !strings.Contains(string(body), `"event":"geo-prophet.quality-alert"`) {
			t.Fatalf("incorrect webhook payload: %s", body)
		}
		return &http.Response{StatusCode: 204, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})
	store := &qualityStoreStub{snapshot: domain.DataStatusSnapshot{QualityRuns: []domain.QualityRun{{
		ID: 7, Dataset: "sales_release", CityCode: "a", Season: "1150901-1150910", FinishedAt: now.Add(-22 * 24 * time.Hour),
	}}}}
	publisher := NewQualityPublisher(store, webhook)
	publisher.now = func() time.Time { return now }
	if err := publisher.Check(context.Background()); err != nil || called != 1 || store.result != nil {
		t.Fatalf("expected one delivered alert, called=%d result=%v check=%v", called, store.result, err)
	}
}

func TestQualityWebhookRejectsInsecureRemoteURL(t *testing.T) {
	if _, err := NewQualityWebhook("http://example.test/quality", ""); err == nil {
		t.Fatal("expected HTTPS-only validation")
	}
}

func TestQualityPublisherKeepsFailedDeliveryPending(t *testing.T) {
	now := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	webhook, _ := NewQualityWebhook("https://example.test/quality", "")
	webhook.client.Transport = qualityTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader("secret response")), Header: make(http.Header)}, nil
	})
	store := &qualityStoreStub{snapshot: domain.DataStatusSnapshot{QualityRuns: []domain.QualityRun{{
		ID: 7, Dataset: "sales_release", CityCode: "a", Season: "1150901-1150910", FinishedAt: now.Add(-22 * 24 * time.Hour),
	}}}}
	publisher := NewQualityPublisher(store, webhook)
	publisher.now = func() time.Time { return now }
	err := publisher.Check(context.Background())
	if err == nil || store.result == nil || strings.Contains(err.Error(), "secret response") {
		t.Fatalf("expected safe retryable error, got check=%v recorded=%v", err, store.result)
	}
}
