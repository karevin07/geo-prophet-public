package transit_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/transit"
)

func TestClientAuthenticatesOnceAndRetriesRateLimits(t *testing.T) {
	tokens, calls := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/token":
			tokens++
			if request.FormValue("client_id") != "id" || request.FormValue("client_secret") != "secret" {
				http.Error(writer, "bad client", http.StatusUnauthorized)
				return
			}
			_, _ = writer.Write([]byte(`{"access_token":"abc","expires_in":86400}`))
		case "/api/v2/Rail/THSR/Station":
			calls++
			if request.Header.Get("Authorization") != "Bearer abc" || request.URL.Query().Get("$format") != "JSON" {
				http.Error(writer, "unauthorized", http.StatusUnauthorized)
				return
			}
			if calls == 1 {
				http.Error(writer, "slow down", http.StatusTooManyRequests)
				return
			}
			_, _ = writer.Write([]byte(`[{"StationID":"0990"}]`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	client := transit.NewClient("id", "secret")
	client.BaseURL, client.TokenURL = server.URL+"/api", server.URL+"/token"
	var waits []time.Duration
	client.Wait = func(_ context.Context, delay time.Duration) error {
		waits = append(waits, delay)
		return nil
	}
	for range 2 {
		var stations []struct{ StationID string }
		if err := client.GetJSON(context.Background(), "v2/Rail/THSR/Station", &stations); err != nil || len(stations) != 1 {
			t.Fatalf("GetJSON = %v, %v", stations, err)
		}
	}
	if tokens != 1 || calls != 3 || len(waits) != 1 {
		t.Errorf("tokens=%d calls=%d waits=%v; want the token reused and one retry", tokens, calls, waits)
	}
}

func TestClientWaitsForTheRateLimitWindow(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		calls++
		// Free members get 5 calls a minute; this window has one left, then none.
		writer.Header().Set("RateLimit-Limit", "5")
		writer.Header().Set("RateLimit-Remaining", map[int]string{1: "1", 2: "0"}[calls])
		writer.Header().Set("RateLimit-Reset", "30")
		if calls == 3 {
			writer.Header().Set("RateLimit-Reset", "12")
			http.Error(writer, "API rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		_, _ = writer.Write([]byte(`[]`))
	}))
	defer server.Close()
	client := transit.NewClient("", "")
	client.BaseURL = server.URL
	var waits []time.Duration
	client.Wait = func(_ context.Context, delay time.Duration) error {
		waits = append(waits, delay.Round(time.Second))
		return nil
	}
	for range 3 {
		if err := client.GetJSON(context.Background(), "v2/Rail/THSR/Station", &[]any{}); err != nil {
			t.Fatalf("GetJSON: %v", err)
		}
	}
	// No wait while calls remain, the window's reset once they run out, and a 429's reset.
	if calls != 4 || len(waits) != 2 || waits[0] != 31*time.Second || waits[1] != 13*time.Second {
		t.Fatalf("calls=%d waits=%v", calls, waits)
	}
}

func TestClientExplainsMissingCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "Valid API Key Required", http.StatusUnauthorized)
	}))
	defer server.Close()
	client := transit.NewClient("", "")
	client.BaseURL = server.URL
	err := client.GetJSON(context.Background(), "v2/Rail/THSR/Station", &[]any{})
	if err == nil || !strings.Contains(err.Error(), "TDX_CLIENT_ID") {
		t.Fatalf("err = %v, want a hint about TDX credentials", err)
	}
}
