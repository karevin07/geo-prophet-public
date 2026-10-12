// Package snapshot writes the static data behind the online demo, which has no API: one
// city's recent sales, which the browser filters and aggregates as the API would, and
// API responses whose parameters are fixed, saved under a key derived from their URL.
package snapshot

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/geo-prophet/geo-prophet/api/internal/geocode"
	"github.com/geo-prophet/geo-prophet/api/internal/lvr"
	"github.com/geo-prophet/geo-prophet/api/internal/usecase"
)

// Version changes when the files change shape; the web app checks it.
const Version = 1

// Sales reads sales the way the API does.
type Sales interface {
	List(context.Context, domain.TransactionFilter) ([]domain.Transaction, error)
	Feed(ctx context.Context, query domain.FeedQuery, latestRunIDs []int64, limit int) ([]domain.Transaction, error)
	LatestRelease(context.Context) (string, []int64, error)
}

type Options struct {
	CityCode string
	// Months of sales to keep, ending with the month of Now.
	Months int
	Now    time.Time
	// CompleteMonth is the API's last fully registered month.
	CompleteMonth func(context.Context) (time.Time, bool, error)
	// API serves the fixed responses.
	API http.Handler
}

// Meta describes a snapshot; it is written last, so a snapshot without it is incomplete.
type Meta struct {
	Version     int       `json:"version"`
	GeneratedAt time.Time `json:"generatedAt"`
	CityCode    string    `json:"cityCode"`
	// From is the first day of sales kept and Through the latest sale (YYYY-MM-DD).
	From    string `json:"from"`
	Through string `json:"through"`
	// CompleteMonth is the last fully registered month (YYYY-MM), empty when unknown.
	CompleteMonth string          `json:"completeMonth"`
	LatestRelease *domain.Release `json:"latestRelease"`
	Region        domain.Region   `json:"region"`
	Sales         int             `json:"sales"`
	Responses     int             `json:"responses"`
}

// maxSales guards against reading a database far larger than one city's few years.
const maxSales = 1_000_000

// salesColumns are the fields of each sale, each stored as an array in sales.json.
var salesColumns = []string{
	"id", "district", "address", "road", "transactionType", "buildingType", "mainUse",
	"totalPrice", "parkingPrice", "buildingPrice", "buildingAreaPing", "pricePerPing", "rooms", "halls", "baths",
	"floorLabel", "totalFloors", "completedAt", "transactedAt", "remarks", "specialReasons", "season",
	"lat", "lng", "geocodePrecision", "isNew",
}

// Write writes meta.json, sales.json and api/<key>.json files into dir.
func Write(ctx context.Context, dir string, sales Sales, options Options) (Meta, error) {
	region, ok := cityRegion(options.CityCode)
	if !ok {
		return Meta{}, fmt.Errorf("unknown city %q", options.CityCode)
	}
	if options.Months < 1 {
		return Meta{}, errors.New("months must be at least 1")
	}
	if err := os.MkdirAll(filepath.Join(dir, "api"), 0o755); err != nil {
		return Meta{}, err
	}
	// Files of an earlier snapshot would be served alongside this one.
	if err := os.Remove(filepath.Join(dir, "meta.json")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Meta{}, err
	}

	from, _ := usecase.MonthRange(options.Now, options.Months)
	// Sales dated after today are typing errors, such as a 2101 sale.
	to := time.Date(options.Now.Year(), options.Now.Month(), options.Now.Day()+1, 0, 0, 0, 0, time.UTC).Add(-time.Nanosecond)
	filter := domain.TransactionFilter{
		MinPrice: 0, MaxPrice: math.MaxInt64, MinArea: 0, MaxArea: math.MaxFloat64,
		IncludeSpecial: true, From: &from, To: &to, Limit: maxSales + 1,
	}
	rows, err := sales.List(ctx, filter)
	if err != nil {
		return Meta{}, fmt.Errorf("read sales: %w", err)
	}
	if len(rows) > maxSales {
		return Meta{}, fmt.Errorf("more than %d sales; use fewer months", maxSales)
	}
	label, runIDs, err := sales.LatestRelease(ctx)
	if err != nil {
		return Meta{}, fmt.Errorf("read the latest release: %w", err)
	}
	fresh := map[string]bool{}
	if len(runIDs) > 0 {
		latest, err := sales.Feed(ctx, domain.FeedQuery{
			Filter: filter, CityCode: options.CityCode, LatestReleaseOnly: true, Sort: domain.SortDateDesc,
		}, runIDs, maxSales)
		if err != nil {
			return Meta{}, fmt.Errorf("read the latest release's sales: %w", err)
		}
		for _, sale := range latest {
			fresh[sale.ID] = true
		}
	}

	meta := Meta{
		Version: Version, GeneratedAt: options.Now.UTC(), CityCode: options.CityCode, From: from.Format(time.DateOnly),
		Region: region,
	}
	// Stored by column, which compresses to two thirds of the same rows stored by row.
	columns := make(map[string][]any, len(salesColumns))
	for _, sale := range rows {
		if sale.CityCode != options.CityCode {
			continue
		}
		for index, value := range saleRow(sale, fresh[sale.ID]) {
			columns[salesColumns[index]] = append(columns[salesColumns[index]], value)
		}
		meta.Sales++
		if day := sale.TransactedAt.Format(time.DateOnly); day > meta.Through {
			meta.Through = day
		}
	}
	if meta.Sales == 0 {
		return Meta{}, fmt.Errorf("no sales in city %s since %s", options.CityCode, meta.From)
	}
	if err = writeJSON(filepath.Join(dir, "sales.json"), columns); err != nil {
		return Meta{}, err
	}

	if release, err := lvr.ParseReleaseLabel(label); err == nil {
		meta.LatestRelease = &domain.Release{Label: label, RegisteredFrom: release.RegisteredFrom, RegisteredTo: release.RegisteredTo}
	}
	complete, known, err := options.CompleteMonth(ctx)
	if err != nil {
		return Meta{}, fmt.Errorf("find the last complete month: %w", err)
	}
	if known {
		meta.CompleteMonth = complete.Format("2006-01")
	}

	for _, target := range Requests(region) {
		if err = saveResponse(dir, options.API, target); err != nil {
			return Meta{}, err
		}
		meta.Responses++
	}
	return meta, writeJSON(filepath.Join(dir, "meta.json"), meta)
}

// Requests lists the API requests saved for the demo, with parameters exactly as the web
// app sends them.
func Requests(region domain.Region) []string {
	city := url.Values{"city": {region.Code}}
	requests := []string{
		"/api/v1/districts/boundaries?" + city.Encode(),
		"/api/v1/rentals/districts?" + city.Encode(),
		"/api/v1/facilities/districts?" + city.Encode(),
	}
	for _, district := range region.Districts {
		requests = append(requests,
			"/api/v1/rentals/trend?"+url.Values{"city": {region.Code}, "district": {district}}.Encode(),
			"/api/v1/presales/projects?"+url.Values{"city": {region.Code}, "district": {district}, "months": {"12"}}.Encode(),
		)
	}
	return requests
}

// Key names the saved response of an API request: a hash of its path and decoded
// parameters in sorted order, so parameter order and percent-encoding do not matter.
// src/lib/api/snapshot-key.ts computes the same.
func Key(target string) (string, error) {
	parsed, err := url.Parse(target)
	if err != nil {
		return "", err
	}
	var pairs []string
	for name, values := range parsed.Query() {
		for _, value := range values {
			pairs = append(pairs, name+"="+value)
		}
	}
	slices.Sort(pairs)
	sum := sha256.Sum256([]byte(parsed.Path + "?" + strings.Join(pairs, "&")))
	return hex.EncodeToString(sum[:])[:20], nil
}

func saveResponse(dir string, api http.Handler, target string) error {
	key, err := Key(target)
	if err != nil {
		return err
	}
	request := httptest.NewRequest(http.MethodGet, target, nil)
	request.Header.Set("Accept", "application/json")
	recorder := httptest.NewRecorder()
	api.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		return fmt.Errorf("%s: status %d: %s", target, recorder.Code, strings.TrimSpace(recorder.Body.String()))
	}
	return os.WriteFile(filepath.Join(dir, "api", key+".json"), recorder.Body.Bytes(), 0o644)
}

func saleRow(sale domain.Transaction, fresh bool) []any {
	road := ""
	if address, ok := geocode.ParseAddress(sale.Address, lvr.Cities[sale.CityCode], sale.District); ok {
		road = address.Road
	}
	var totalFloors, completedAt any
	if sale.TotalFloors > 0 {
		totalFloors = sale.TotalFloors
	}
	if sale.CompletedAt != nil {
		completedAt = sale.CompletedAt.Format(time.DateOnly)
	}
	reasons := sale.SpecialReasons
	if reasons == nil {
		reasons = []string{}
	}
	isNew := 0
	if fresh {
		isNew = 1
	}
	return []any{
		sale.ID, sale.District, sale.Address, road, sale.TransactionType, sale.BuildingType, sale.MainUse,
		sale.TotalPrice, sale.ParkingPrice, sale.BuildingPrice, round(sale.BuildingAreaPing, 2), round(sale.PricePerPing, 2),
		sale.Rooms, sale.Halls, sale.Baths, sale.FloorLabel, totalFloors, completedAt, sale.TransactedAt.Format(time.DateOnly),
		sale.Remarks, reasons, sale.Season, round(sale.Lat, 6), round(sale.Lng, 6), sale.GeocodePrecision, isNew,
	}
}

func round(value float64, places int) float64 {
	scale := math.Pow(10, float64(places))
	return math.Round(value*scale) / scale
}

func cityRegion(cityCode string) (domain.Region, bool) {
	for _, region := range usecase.Regions() {
		if region.Code == cityCode {
			return region, true
		}
	}
	return domain.Region{}, false
}

func writeJSON(path string, value any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o644)
}
