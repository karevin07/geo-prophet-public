// Package opendata downloads files from Taiwanese government open data portals.
package opendata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultDataGovBaseURL = "https://data.gov.tw/api/v2/rest/dataset/"
	defaultLVRBaseURL     = "https://plvr.land.moi.gov.tw"
	// The Taiwan extract is about 330 MB and refreshed daily.
	defaultOSMExtractURL = "https://download.geofabrik.de/asia/taiwan-latest.osm.pbf"
	userAgent            = "geo-prophet-importer/1.0"
)

type Client struct {
	HTTP           *http.Client
	DataGovBaseURL string
	LVRBaseURL     string
	OSMURL         string
}

func NewClient() *Client {
	return &Client{
		// The OpenStreetMap extract exceeds 300MB and downloads at about 150 KB/s from
		// abroad; the timeout guards against stalled transfers.
		HTTP:           &http.Client{Timeout: 90 * time.Minute},
		DataGovBaseURL: defaultDataGovBaseURL,
		LVRBaseURL:     defaultLVRBaseURL,
		OSMURL:         defaultOSMExtractURL,
	}
}

type datasetResponse struct {
	Success bool `json:"success"`
	Result  struct {
		Distribution []struct {
			Description string `json:"resourceDescription"`
			Format      string `json:"resourceFormat"`
			DownloadURL string `json:"resourceDownloadUrl"`
		} `json:"distribution"`
	} `json:"result"`
}

// LatestCSVURL returns the last CSV distribution of a data.gov.tw dataset. Datasets that
// publish one file per month (e.g. Taoyuan) list them oldest first.
func (client *Client) LatestCSVURL(ctx context.Context, datasetID string) (string, error) {
	body, err := client.Open(ctx, client.DataGovBaseURL+url.PathEscape(datasetID))
	if err != nil {
		return "", err
	}
	defer body.Close()
	var response datasetResponse
	if err = json.NewDecoder(body).Decode(&response); err != nil {
		return "", fmt.Errorf("decode dataset %s: %w", datasetID, err)
	}
	distributions := response.Result.Distribution
	for index := len(distributions) - 1; index >= 0; index-- {
		if strings.EqualFold(distributions[index].Format, "CSV") && distributions[index].DownloadURL != "" {
			return distributions[index].DownloadURL, nil
		}
	}
	return "", fmt.Errorf("dataset %s has no CSV distribution", datasetID)
}

// OSMExtractURL returns the Taiwan OpenStreetMap extract (Geofabrik).
func (client *Client) OSMExtractURL() string { return client.OSMURL }

// LVRSeasonURL returns the archive of one season (e.g. 115S2) with every city's CSV files.
func (client *Client) LVRSeasonURL(season string) string {
	query := url.Values{"season": {season}, "type": {"zip"}, "fileName": {"lvr_landcsv.zip"}}
	return client.LVRBaseURL + "/DownloadSeason?" + query.Encode()
}

// LVRCurrentURL returns the archive of the latest 10-day release (本期資料).
func (client *Client) LVRCurrentURL() string {
	query := url.Values{"type": {"zip"}, "fileName": {"lvr_landcsv.zip"}}
	return client.LVRBaseURL + "/Download?" + query.Encode()
}

// Open issues a GET request and returns the body of a successful response.
func (client *Client) Open(ctx context.Context, target string) (io.ReadCloser, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	request.Header.Set("User-Agent", userAgent)
	response, err := client.HTTP.Do(request)
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", target, err)
	}
	if response.StatusCode != http.StatusOK {
		response.Body.Close()
		return nil, fmt.Errorf("get %s: unexpected status %s", target, response.Status)
	}
	return response.Body, nil
}
