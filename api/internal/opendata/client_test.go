package opendata_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/geo-prophet/geo-prophet/api/internal/opendata"
)

func TestLVRURLs(t *testing.T) {
	client := opendata.NewClient()
	if got := client.LVRSeasonURL("115S2"); got != "https://plvr.land.moi.gov.tw/DownloadSeason?fileName=lvr_landcsv.zip&season=115S2&type=zip" {
		t.Fatalf("unexpected season URL %s", got)
	}
	if got := client.LVRCurrentURL(); got != "https://plvr.land.moi.gov.tw/Download?fileName=lvr_landcsv.zip&type=zip" {
		t.Fatalf("unexpected current URL %s", got)
	}
}

func TestLatestCSVURLPicksLastCSVDistribution(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/157689" {
			http.NotFound(writer, request)
			return
		}
		_, _ = writer.Write([]byte(`{"success":true,"result":{"distribution":[
			{"resourceFormat":"CSV","resourceDownloadUrl":"https://example.test/115-07.csv"},
			{"resourceFormat":"CSV","resourceDownloadUrl":"https://example.test/115-08.csv"},
			{"resourceFormat":"JSON","resourceDownloadUrl":"https://example.test/115-08.json"}]}}`))
	}))
	defer server.Close()
	client := opendata.NewClient()
	client.DataGovBaseURL = server.URL + "/"

	got, err := client.LatestCSVURL(context.Background(), "157689")
	if err != nil || got != "https://example.test/115-08.csv" {
		t.Fatalf("LatestCSVURL = %q, %v", got, err)
	}
	if _, err = client.LatestCSVURL(context.Background(), "missing"); err == nil {
		t.Fatal("expected an error for a missing dataset")
	}
}
