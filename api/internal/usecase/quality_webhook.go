package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
)

type QualityAlertStore interface {
	QualitySnapshot(context.Context) (domain.DataStatusSnapshot, error)
	QueueQualityAlerts(context.Context, []domain.QualityAlert) error
	ResolveInactiveQualityAlerts(context.Context, []domain.QualityAlert) error
	PendingQualityAlerts(context.Context) ([]domain.QualityAlert, error)
	FinishQualityAlert(context.Context, domain.QualityAlert, error) error
}

// QualityWebhook sends generic JSON events. The URL is configured by the operator,
// never supplied by a browser; production destinations must use HTTPS.
type QualityWebhook struct {
	url, token string
	client     *http.Client
}

func NewQualityWebhook(target, token string) (*QualityWebhook, error) {
	parsed, err := url.ParseRequestURI(target)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return nil, errors.New("QUALITY_WEBHOOK_URL must be an absolute HTTPS URL")
	}
	host := parsed.Hostname()
	ip := net.ParseIP(host)
	loopback := host == "localhost" || (ip != nil && ip.IsLoopback())
	if parsed.Scheme != "https" && !(parsed.Scheme == "http" && loopback) {
		return nil, errors.New("QUALITY_WEBHOOK_URL requires HTTPS except for loopback testing")
	}
	return &QualityWebhook{url: target, token: token, client: &http.Client{
		Timeout:       10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

func (webhook *QualityWebhook) Send(ctx context.Context, alert domain.QualityAlert) error {
	body, err := json.Marshal(struct {
		Event string              `json:"event"`
		Alert domain.QualityAlert `json:"alert"`
	}{Event: "geo-prophet.quality-alert", Alert: alert})
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, webhook.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	if webhook.token != "" {
		request.Header.Set("Authorization", "Bearer "+webhook.token)
	}
	response, err := webhook.client.Do(request)
	if err != nil {
		// net/http errors can contain the configured URL and its query string, which
		// may be a secret. Keep diagnostics class-only in the database and logs.
		return fmt.Errorf("send quality webhook: %T", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("quality webhook returned HTTP %d", response.StatusCode)
	}
	return nil
}

type QualityPublisher struct {
	store   QualityAlertStore
	webhook *QualityWebhook
	now     func() time.Time
}

func NewQualityPublisher(store QualityAlertStore, webhook *QualityWebhook) *QualityPublisher {
	return &QualityPublisher{store: store, webhook: webhook, now: time.Now}
}

// Check queues each active condition once and retries due deliveries. The database
// makes page refreshes and importer restarts idempotent; delivery is at-least-once if
// the process stops after the receiver accepts but before sent_at is persisted.
func (publisher *QualityPublisher) Check(ctx context.Context) error {
	snapshot, err := publisher.store.QualitySnapshot(ctx)
	if err != nil {
		return err
	}
	active := AssessQuality(snapshot, publisher.now())
	if err := publisher.store.QueueQualityAlerts(ctx, active.Alerts); err != nil {
		return err
	}
	if err := publisher.store.ResolveInactiveQualityAlerts(ctx, active.Alerts); err != nil {
		return err
	}
	pending, err := publisher.store.PendingQualityAlerts(ctx)
	if err != nil {
		return err
	}
	current := make(map[string]bool, len(active.Alerts))
	for _, alert := range active.Alerts {
		current[qualityAlertKey(alert)] = true
	}
	var failures []error
	for _, alert := range pending {
		if !current[qualityAlertKey(alert)] {
			continue // the condition recovered before a retry
		}
		deliveryErr := publisher.webhook.Send(ctx, alert)
		if err := publisher.store.FinishQualityAlert(ctx, alert, deliveryErr); err != nil {
			failures = append(failures, err)
		}
		if deliveryErr != nil {
			failures = append(failures, deliveryErr)
		}
	}
	return errors.Join(failures...)
}

func qualityAlertKey(alert domain.QualityAlert) string {
	return fmt.Sprintf("%d|%s", alert.RunID, strings.ToLower(alert.Code))
}
