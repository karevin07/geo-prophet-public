package transit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	DefaultBaseURL  = "https://tdx.transportdata.tw/api/basic"
	DefaultTokenURL = "https://tdx.transportdata.tw/auth/realms/TDXConnect/protocol/openid-connect/token"
	userAgent       = "geo-prophet-importer/1.0"
	maxAttempts     = 4
	// defaultReset is how long to back off from a 429 that does not say.
	defaultReset = time.Minute
)

// Getter fetches one TDX API path, such as v2/Rail/THSR/Station, as JSON.
type Getter interface {
	GetJSON(ctx context.Context, path string, into any) error
}

// Client calls TDX with an OAuth client-credentials token. Without credentials it calls
// anonymously, which TDX allows only a few times a day per IP. It follows TDX's rate
// limit headers: a free member may call 5 times a minute, so an import of about 30 calls
// takes several minutes.
type Client struct {
	HTTP         *http.Client
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	// Wait is used for rate limits and retries; tests replace it.
	Wait   func(context.Context, time.Duration) error
	Logger *slog.Logger

	mu      sync.Mutex
	token   string
	expires time.Time
	// resumeAt is when the rate limit window resets after its calls ran out.
	resumeAt time.Time
}

func NewClient(clientID, clientSecret string) *Client {
	return &Client{
		HTTP: &http.Client{Timeout: 2 * time.Minute}, BaseURL: DefaultBaseURL, TokenURL: DefaultTokenURL,
		ClientID: clientID, ClientSecret: clientSecret, Wait: sleep,
	}
}

// GetJSON waits out an exhausted rate limit before calling, retries a 429 once the limit
// resets, and retries server errors with a growing delay.
func (client *Client) GetJSON(ctx context.Context, path string, into any) error {
	target := strings.TrimRight(client.BaseURL, "/") + "/" + strings.TrimLeft(path, "/") + "?%24format=JSON"
	for attempt := 1; ; attempt++ {
		if delay := time.Until(client.resumeAt); delay > 0 {
			client.log("waiting for the TDX rate limit to reset", "seconds", int(delay.Round(time.Second).Seconds()), "next", path)
			if err := client.Wait(ctx, delay); err != nil {
				return err
			}
			client.resumeAt = time.Time{}
		}
		retry, err := client.get(ctx, target, into)
		if err == nil || !retry || attempt == maxAttempts {
			return err
		}
		// A 429 sets resumeAt, so the loop waits for the reset; otherwise back off.
		if time.Until(client.resumeAt) <= 0 {
			if err = client.Wait(ctx, time.Duration(attempt)*2*time.Second); err != nil {
				return err
			}
		}
	}
}

func (client *Client) log(message string, args ...any) {
	if client.Logger != nil {
		client.Logger.Info(message, args...)
	}
}

// trackRateLimit records when calls may resume once none remain in the window, from
// RateLimit-Remaining and RateLimit-Reset (seconds).
func (client *Client) trackRateLimit(header http.Header, limited bool) {
	remaining, err := strconv.Atoi(header.Get("RateLimit-Remaining"))
	if !limited && (err != nil || remaining > 0) {
		return
	}
	reset := defaultReset
	if seconds, err := strconv.Atoi(header.Get("RateLimit-Reset")); err == nil && seconds >= 0 {
		reset = time.Duration(seconds+1) * time.Second
	}
	client.resumeAt = time.Now().Add(reset)
}

func (client *Client) get(ctx context.Context, target string, into any) (retry bool, err error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return false, fmt.Errorf("build request: %w", err)
	}
	request.Header.Set("User-Agent", userAgent)
	if client.ClientID != "" {
		token, err := client.accessToken(ctx)
		if err != nil {
			return false, err
		}
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := client.HTTP.Do(request)
	if err != nil {
		return true, fmt.Errorf("get %s: %w", target, err)
	}
	defer response.Body.Close()
	client.trackRateLimit(response.Header, response.StatusCode == http.StatusTooManyRequests)
	switch {
	case response.StatusCode == http.StatusOK:
	case response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500:
		return true, fmt.Errorf("get %s: %s", target, response.Status)
	case response.StatusCode == http.StatusUnauthorized && client.ClientID == "":
		return false, fmt.Errorf("get %s: %s; set TDX_CLIENT_ID and TDX_CLIENT_SECRET", target, response.Status)
	default:
		body, _ := io.ReadAll(io.LimitReader(response.Body, 300))
		return false, fmt.Errorf("get %s: %s: %s", target, response.Status, strings.TrimSpace(string(body)))
	}
	if err = json.NewDecoder(response.Body).Decode(into); err != nil {
		return false, fmt.Errorf("decode %s: %w", target, err)
	}
	return false, nil
}

// accessToken returns a cached token, renewing it a minute before it expires.
func (client *Client) accessToken(ctx context.Context) (string, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.token != "" && time.Now().Before(client.expires) {
		return client.token, nil
	}
	form := url.Values{"grant_type": {"client_credentials"}, "client_id": {client.ClientID}, "client_secret": {client.ClientSecret}}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("build token request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("User-Agent", userAgent)
	response, err := client.HTTP.Do(request)
	if err != nil {
		return "", fmt.Errorf("request TDX token: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("request TDX token: %s; check TDX_CLIENT_ID and TDX_CLIENT_SECRET", response.Status)
	}
	var token struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err = json.NewDecoder(response.Body).Decode(&token); err != nil || token.AccessToken == "" {
		return "", fmt.Errorf("decode TDX token: %v", err)
	}
	client.token = token.AccessToken
	client.expires = time.Now().Add(time.Duration(token.ExpiresIn)*time.Second - time.Minute)
	return client.token, nil
}

func sleep(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
