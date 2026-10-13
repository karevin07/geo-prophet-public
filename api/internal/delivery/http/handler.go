package http

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/geo-prophet/geo-prophet/api/internal/usecase"
	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// defaultWindowMonths is the default date window; 實價登錄 lags transactions by 1-3 months.
const defaultWindowMonths = 12

// maxSearchResults caps the search box's results, communities included.
const maxSearchResults = 10

type Handler struct {
	service     *usecase.TransactionService
	places      *usecase.PlaceService
	feed        *usecase.FeedService
	investment  *usecase.InvestmentService
	workbench   *usecase.InvestmentWorkbenchService
	liquidity   *usecase.InvestmentLiquidityService
	rentals     *usecase.RentalService
	estimates   *usecase.EstimateService
	presales    *usecase.PresaleService
	communities *usecase.CommunityService
	facilities  *usecase.FacilityService
	transit     *usecase.TransitService
	dataStatus  *usecase.DataStatusService
	admin       *usecase.AdminService
	now         func() time.Time
}

// Services are the use cases the handlers call. Workbench, Liquidity, Rentals,
// Estimates, Presales, Communities, Facilities, DataStatus and Admin need a database;
// when nil, their endpoints answer 503, and without Communities sales, hexagons and
// searches carry no community links.
type Services struct {
	Transactions *usecase.TransactionService
	Places       *usecase.PlaceService
	Feed         *usecase.FeedService
	Investment   *usecase.InvestmentService
	Workbench    *usecase.InvestmentWorkbenchService
	Liquidity    *usecase.InvestmentLiquidityService
	Rentals      *usecase.RentalService
	Estimates    *usecase.EstimateService
	Presales     *usecase.PresaleService
	Communities  *usecase.CommunityService
	Facilities   *usecase.FacilityService
	Transit      *usecase.TransitService
	DataStatus   *usecase.DataStatusService
	Admin        *usecase.AdminService
}

func NewHandler(services Services) *Handler {
	return &Handler{
		service: services.Transactions, places: services.Places, feed: services.Feed, investment: services.Investment, workbench: services.Workbench, liquidity: services.Liquidity, rentals: services.Rentals, estimates: services.Estimates, presales: services.Presales, communities: services.Communities, facilities: services.Facilities, transit: services.Transit,
		dataStatus: services.DataStatus, admin: services.Admin, now: time.Now,
	}
}

func (handler *Handler) InvestmentAreas(c *gin.Context) {
	query, ok := handler.investmentQuery(c)
	if !ok {
		return
	}
	report, err := handler.investment.CompareAreas(c.Request.Context(), query)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": report})
}

func (handler *Handler) InvestmentWorkbench(c *gin.Context) {
	if handler.workbench == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"code": "unavailable", "message": "workbench requires imported sales and rentals"}})
		return
	}
	query, ok := handler.investmentQuery(c)
	if !ok {
		return
	}
	report, err := handler.workbench.Compare(c.Request.Context(), query)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": report})
}

func (handler *Handler) InvestmentLiquidity(c *gin.Context) {
	if handler.liquidity == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"code": "unavailable", "message": "liquidity research requires imported sales", "requestId": requestid.Get(c)}})
		return
	}
	query, ok := handler.investmentQuery(c)
	if !ok {
		return
	}
	report, err := handler.liquidity.Compare(c.Request.Context(), query)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": report})
}

func (handler *Handler) investmentQuery(c *gin.Context) (domain.InvestmentQuery, bool) {
	minArea, err := floatQuery(c, "min_area", 15)
	if err != nil {
		handler.writeError(c, err)
		return domain.InvestmentQuery{}, false
	}
	maxArea, err := floatQuery(c, "max_area", 45)
	if err != nil {
		handler.writeError(c, err)
		return domain.InvestmentQuery{}, false
	}
	minAge, err := integerQuery(c, "min_age", 5)
	if err != nil {
		handler.writeError(c, err)
		return domain.InvestmentQuery{}, false
	}
	maxAge, err := integerQuery(c, "max_age", 30)
	if err != nil {
		handler.writeError(c, err)
		return domain.InvestmentQuery{}, false
	}
	return domain.InvestmentQuery{
		CityCode: c.DefaultQuery("city", "a"), Category: c.DefaultQuery("type", "住宅大樓"),
		MinArea: minArea, MaxArea: maxArea, MinAge: minAge, MaxAge: maxAge,
	}, true
}

// RentalDistricts compares rents and gross yields across a city's districts.
func (handler *Handler) RentalDistricts(c *gin.Context) {
	query, ok := handler.rentalQuery(c)
	if !ok {
		return
	}
	report, err := handler.rentals.CompareDistricts(c.Request.Context(), query)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": report})
}

// RentLayer returns rents and gross yields of the H3 cells inside bbox, for the map.
func (handler *Handler) RentLayer(c *gin.Context) {
	if handler.rentals == nil {
		handler.rentalsUnavailable(c)
		return
	}
	filter, err := handler.parseFilter(c)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	resolution, err := integerQuery(c, "resolution", 9)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	layer, err := handler.rentals.Layer(c.Request.Context(), filter, resolution)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": layer})
}

// RentalTrend returns a city's and optionally a district's monthly rent per ping.
func (handler *Handler) RentalTrend(c *gin.Context) {
	query, ok := handler.rentalQuery(c)
	if !ok {
		return
	}
	trend, err := handler.rentals.Trend(c.Request.Context(), query, c.Query("district"))
	if err != nil {
		handler.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": trend})
}

// StationRent compares rents and gross yields within each ring of a station.
func (handler *Handler) StationRent(c *gin.Context) {
	if handler.rentals == nil {
		handler.rentalsUnavailable(c)
		return
	}
	includeSocial, err := boolQuery(c, "include_social")
	if err != nil {
		handler.writeError(c, err)
		return
	}
	rent, err := handler.rentals.StationRent(c.Request.Context(), c.Param("id"), includeSocial)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rent})
}

// Estimate values a home from comparable sales nearby.
func (handler *Handler) Estimate(c *gin.Context) {
	if handler.estimates == nil {
		handler.estimatesUnavailable(c)
		return
	}
	lat, lng, ok := handler.point(c)
	if !ok {
		return
	}
	area, err := floatQuery(c, "area", 0)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	floor, err := integerQuery(c, "floor", 0)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	totalFloors, err := integerQuery(c, "total_floors", 0)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	parking, err := boolQuery(c, "parking")
	if err != nil {
		handler.writeError(c, err)
		return
	}
	query := domain.EstimateQuery{
		Address: c.Query("address"), Lat: lat, Lng: lng, Category: c.Query("type"), AreaPing: area,
		Floor: floor, TotalFloors: totalFloors, Parking: parking,
	}
	if c.Query("age") != "" {
		age, err := integerQuery(c, "age", 0)
		if err != nil {
			handler.writeError(c, err)
			return
		}
		query.Age = &age
	}
	estimate, err := handler.estimates.Estimate(c.Request.Context(), query)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": estimate})
}

// EstimateBuilding summarises a building's recorded sales to fill in the estimate form.
func (handler *Handler) EstimateBuilding(c *gin.Context) {
	if handler.estimates == nil {
		handler.estimatesUnavailable(c)
		return
	}
	lat, lng, ok := handler.point(c)
	if !ok {
		return
	}
	profile, err := handler.estimates.Building(c.Request.Context(), c.Query("address"), lat, lng)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	if handler.communities != nil {
		if profile.Community, err = handler.communities.ByDoor(c.Request.Context(), c.Query("address")); err != nil {
			handler.writeError(c, err)
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": profile})
}

// PresaleProjects lists a city's presale projects with recent contracts.
func (handler *Handler) PresaleProjects(c *gin.Context) {
	if handler.presales == nil {
		handler.unavailableWithoutDatabase(c, "presales")
		return
	}
	months, err := integerQuery(c, "months", 0)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	var near *domain.NearPoint
	if c.Query("lat") != "" || c.Query("lng") != "" {
		lat, lng, ok := handler.point(c)
		if !ok {
			return
		}
		radius, err := floatQuery(c, "radius", 800)
		if err != nil {
			handler.writeError(c, err)
			return
		}
		near = &domain.NearPoint{Lat: lat, Lng: lng, RadiusM: radius}
	}
	var bounds *domain.Bounds
	if bbox := c.Query("bbox"); bbox != "" {
		parsed, err := parseBBox(bbox)
		if err != nil {
			handler.writeError(c, err)
			return
		}
		bounds = &parsed
	}
	projects, err := handler.presales.Projects(c.Request.Context(), c.DefaultQuery("city", "a"), c.Query("district"), months, near, bounds)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": projects})
}

// PresaleProject describes one presale project, identified by its key.
func (handler *Handler) PresaleProject(c *gin.Context) {
	if handler.presales == nil {
		handler.unavailableWithoutDatabase(c, "presales")
		return
	}
	project, err := handler.presales.Project(c.Request.Context(), c.Query("key"))
	if err != nil {
		handler.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": project})
}

func (handler *Handler) unavailableWithoutDatabase(c *gin.Context, what string) {
	c.JSON(http.StatusServiceUnavailable, errorResponse{Error: apiError{Code: "not_available", Message: what + " require a database", RequestID: requestid.Get(c)}})
}

// point parses the required lat and lng, answering the request itself when it cannot.
func (handler *Handler) point(c *gin.Context) (float64, float64, bool) {
	lat, latErr := strconv.ParseFloat(c.Query("lat"), 64)
	lng, lngErr := strconv.ParseFloat(c.Query("lng"), 64)
	if latErr != nil || lngErr != nil {
		handler.writeError(c, fmt.Errorf("%w: lat and lng are required numbers", usecase.ErrInvalidInput))
		return 0, 0, false
	}
	return lat, lng, true
}

func (handler *Handler) estimatesUnavailable(c *gin.Context) {
	c.JSON(http.StatusServiceUnavailable, errorResponse{Error: apiError{Code: "not_available", Message: "estimates require a database", RequestID: requestid.Get(c)}})
}

// parseBBox reads west,south,east,north.
func parseBBox(bbox string) (domain.Bounds, error) {
	parts := strings.Split(bbox, ",")
	if len(parts) != 4 {
		return domain.Bounds{}, fmt.Errorf("%w: bbox must contain west,south,east,north", usecase.ErrInvalidInput)
	}
	values := make([]float64, 4)
	for index, part := range parts {
		value, err := strconv.ParseFloat(part, 64)
		if err != nil {
			return domain.Bounds{}, fmt.Errorf("%w: bbox contains a non-number", usecase.ErrInvalidInput)
		}
		values[index] = value
	}
	return domain.Bounds{West: values[0], South: values[1], East: values[2], North: values[3]}, nil
}

// Facilities lists the facilities of some categories on a map.
func (handler *Handler) Facilities(c *gin.Context) {
	if handler.facilities == nil {
		handler.unavailableWithoutDatabase(c, "facilities")
		return
	}
	bounds, err := parseBBox(c.Query("bbox"))
	if err != nil {
		handler.writeError(c, err)
		return
	}
	list, err := handler.facilities.Facilities(c.Request.Context(), bounds, categoriesQuery(c))
	if err != nil {
		handler.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": list})
}

func categoriesQuery(c *gin.Context) []string {
	var categories []string
	for _, category := range strings.Split(c.Query("categories"), ",") {
		if category = strings.TrimSpace(category); category != "" {
			categories = append(categories, category)
		}
	}
	return categories
}

// NearbyFacilities summarises every facility category around a point.
func (handler *Handler) NearbyFacilities(c *gin.Context) {
	if handler.facilities == nil {
		handler.unavailableWithoutDatabase(c, "facilities")
		return
	}
	lat, lng, ok := handler.point(c)
	if !ok {
		return
	}
	radius, err := floatQuery(c, "radius", 500)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	nearby, err := handler.facilities.Nearby(c.Request.Context(), lat, lng, radius)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": nearby})
}

// DistrictFacilities counts a city's facilities by district for densities per km².
func (handler *Handler) DistrictFacilities(c *gin.Context) {
	if handler.facilities == nil {
		handler.unavailableWithoutDatabase(c, "facilities")
		return
	}
	result, err := handler.facilities.Districts(c.Request.Context(), c.DefaultQuery("city", "a"))
	if err != nil {
		handler.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

// FacilityDensity scores the H3 cells of a map by the facilities within a short walk.
func (handler *Handler) FacilityDensity(c *gin.Context) {
	if handler.facilities == nil {
		handler.unavailableWithoutDatabase(c, "facilities")
		return
	}
	bounds, err := parseBBox(c.Query("bbox"))
	if err != nil {
		handler.writeError(c, err)
		return
	}
	resolution, err := integerQuery(c, "res", 9)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	cells, err := handler.facilities.Density(c.Request.Context(), bounds, resolution, categoriesQuery(c))
	if err != nil {
		handler.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": cells})
}

// rentalQuery parses the rental filters, answering the request itself when it cannot.
func (handler *Handler) rentalQuery(c *gin.Context) (domain.RentalQuery, bool) {
	if handler.rentals == nil {
		handler.rentalsUnavailable(c)
		return domain.RentalQuery{}, false
	}
	minArea, err := floatQuery(c, "min_area", 0)
	if err != nil {
		handler.writeError(c, err)
		return domain.RentalQuery{}, false
	}
	maxArea, err := floatQuery(c, "max_area", 0)
	if err != nil {
		handler.writeError(c, err)
		return domain.RentalQuery{}, false
	}
	includeSocial, err := boolQuery(c, "include_social")
	if err != nil {
		handler.writeError(c, err)
		return domain.RentalQuery{}, false
	}
	return domain.RentalQuery{
		CityCode: c.DefaultQuery("city", "a"), Category: c.Query("type"),
		MinArea: minArea, MaxArea: maxArea, IncludeSocialHousing: includeSocial,
	}, true
}

func (handler *Handler) rentalsUnavailable(c *gin.Context) {
	c.JSON(http.StatusServiceUnavailable, errorResponse{Error: apiError{Code: "not_available", Message: "rental data requires a database", RequestID: requestid.Get(c)}})
}

func (handler *Handler) ListTransactions(c *gin.Context) {
	filter, err := handler.parseFilter(c)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	transactions, err := handler.service.List(c.Request.Context(), filter)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, collectionResponse[domain.Transaction]{Data: transactions, Meta: responseMeta{Count: len(transactions)}})
}

func (handler *Handler) AggregateH3(c *gin.Context) {
	filter, err := handler.parseFilter(c)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	resolution, err := integerQuery(c, "resolution", 9)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	aggregates, err := handler.service.AggregateH3(c.Request.Context(), filter, resolution)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, collectionResponse[domain.H3Aggregate]{Data: aggregates, Meta: responseMeta{Count: len(aggregates)}})
}

func (handler *Handler) Trends(c *gin.Context) {
	filter, err := handler.parseFilter(c)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	trends, err := handler.service.Trends(c.Request.Context(), filter)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, collectionResponse[domain.TrendPoint]{Data: trends, Meta: responseMeta{Count: len(trends)}})
}

func (handler *Handler) Summary(c *gin.Context) {
	filter, err := handler.parseFilter(c)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	summary, err := handler.service.Summary(c.Request.Context(), filter)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": summary})
}

func (handler *Handler) Analysis(c *gin.Context) {
	filter, err := handler.parseFilter(c)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	if c.Query("from") == "" {
		from, _ := usecase.MonthRange(handler.now(), 60)
		filter.From = &from
	}
	analysis, err := handler.service.Analysis(c.Request.Context(), filter)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": analysis})
}

func (handler *Handler) CompareDistricts(c *gin.Context) {
	filter, err := handler.parseFilter(c)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	if c.Query("from") == "" {
		from, _ := usecase.MonthRange(handler.now(), 36)
		filter.From = &from
	}
	city := c.DefaultQuery("city", "a")
	var districts []string
	if selected := c.Query("districts"); selected != "" {
		districts = strings.Split(selected, ",")
	}
	comparison, err := handler.service.CompareDistricts(c.Request.Context(), domain.DistrictComparisonQuery{
		Filter: filter, CityCode: city, Districts: districts,
	})
	if err != nil {
		handler.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": comparison})
}

// CellDetail describes one hexagon for the map's hover card, using the map's filters.
func (handler *Handler) CellDetail(c *gin.Context) {
	filter, err := handler.parseFilter(c)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	detail, err := handler.service.CellDetail(c.Request.Context(), filter, c.Query("hex"))
	if err != nil {
		handler.writeError(c, err)
		return
	}
	if handler.communities == nil {
		detail.Communities = []domain.CommunityRef{}
	} else if detail.Communities, err = handler.communities.Describe(c.Request.Context(), detail.Communities); err != nil {
		handler.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": detail})
}

// DistrictBoundaries serves a city's approximate district areas as GeoJSON. They change
// only when door numbers are re-imported, so browsers may cache them for an hour.
func (handler *Handler) DistrictBoundaries(c *gin.Context) {
	boundaries, err := handler.service.DistrictBoundaries(c.Request.Context(), c.DefaultQuery("city", "a"))
	if err != nil {
		handler.writeError(c, err)
		return
	}
	features := make([]gin.H, 0, len(boundaries))
	for _, boundary := range boundaries {
		features = append(features, gin.H{
			"type":       "Feature",
			"properties": gin.H{"district": boundary.District, "cells": boundary.Cells},
			"geometry":   gin.H{"type": "MultiPolygon", "coordinates": boundary.Coordinates},
		})
	}
	c.Header("Cache-Control", "public, max-age=3600")
	c.JSON(http.StatusOK, gin.H{"type": "FeatureCollection", "features": features})
}

func (handler *Handler) SearchPlaces(c *gin.Context) {
	places, err := handler.places.Search(c.Request.Context(), c.Query("q"))
	if err != nil {
		handler.writeError(c, err)
		return
	}
	// Communities come first: a name or door number names one building better than a road.
	if handler.communities != nil {
		communities, err := handler.communities.Search(c.Request.Context(), c.Query("q"), places)
		if err != nil {
			handler.writeError(c, err)
			return
		}
		places = append(communities, places...)
		if len(places) > maxSearchResults {
			places = places[:maxSearchResults]
		}
	}
	c.JSON(http.StatusOK, collectionResponse[domain.Place]{Data: places, Meta: responseMeta{Count: len(places)}})
}

func (handler *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (handler *Handler) Ready(c *gin.Context) {
	if err := handler.service.Ready(c.Request.Context()); err != nil {
		c.JSON(http.StatusServiceUnavailable, errorResponse{Error: apiError{Code: "not_ready", Message: "dependency check failed", RequestID: requestid.Get(c)}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}

func (handler *Handler) writeError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	code := "internal_error"
	message := "internal server error"
	switch {
	case errors.Is(err, usecase.ErrInvalidInput):
		status, code, message = http.StatusBadRequest, "invalid_request", err.Error()
	case errors.Is(err, usecase.ErrNotFound):
		status, code, message = http.StatusNotFound, "not_found", err.Error()
	case errors.Is(err, usecase.ErrConflict):
		status, code, message = http.StatusConflict, "conflict", err.Error()
	default:
		// The client only sees a generic message, so keep the cause for the request ID.
		slog.Error("request failed", "request_id", requestid.Get(c), "path", c.FullPath(), "error", err)
	}
	c.JSON(status, errorResponse{Error: apiError{Code: code, Message: message, RequestID: requestid.Get(c)}})
}

func (handler *Handler) parseFilter(c *gin.Context) (domain.TransactionFilter, error) {
	minPrice, err := int64Query(c, "min_price", 0)
	if err != nil {
		return domain.TransactionFilter{}, err
	}
	maxPrice, err := int64Query(c, "max_price", math.MaxInt64)
	if err != nil {
		return domain.TransactionFilter{}, err
	}
	minArea, err := floatQuery(c, "min_area", 0)
	if err != nil {
		return domain.TransactionFilter{}, err
	}
	maxArea, err := floatQuery(c, "max_area", math.MaxFloat64)
	if err != nil {
		return domain.TransactionFilter{}, err
	}
	limit, err := integerQuery(c, "limit", 500)
	if err != nil {
		return domain.TransactionFilter{}, err
	}
	includeSpecial, err := boolQuery(c, "include_special")
	if err != nil {
		return domain.TransactionFilter{}, err
	}
	excludeNew, err := boolQuery(c, "exclude_new")
	if err != nil {
		return domain.TransactionFilter{}, err
	}

	filter := domain.TransactionFilter{
		MinPrice: minPrice, MaxPrice: maxPrice, MinArea: minArea, MaxArea: maxArea, Limit: limit, IncludeSpecial: includeSpecial,
		ExcludeNewBuildings: excludeNew,
	}
	for _, category := range strings.Split(c.Query("types"), ",") {
		if category = strings.TrimSpace(category); category != "" {
			filter.Categories = append(filter.Categories, category)
		}
	}
	if bbox := c.Query("bbox"); bbox != "" {
		bounds, err := parseBBox(bbox)
		if err != nil {
			return domain.TransactionFilter{}, err
		}
		filter.Bounds = &bounds
	}

	from, to := usecase.MonthRange(handler.now(), defaultWindowMonths)
	if value := c.Query("from"); value != "" {
		if from, err = time.Parse("2006-01", value); err != nil {
			return domain.TransactionFilter{}, fmt.Errorf("%w: from must use YYYY-MM", usecase.ErrInvalidInput)
		}
	}
	if value := c.Query("to"); value != "" {
		month, err := time.Parse("2006-01", value)
		if err != nil {
			return domain.TransactionFilter{}, fmt.Errorf("%w: to must use YYYY-MM", usecase.ErrInvalidInput)
		}
		to = month.AddDate(0, 1, 0).Add(-time.Nanosecond)
	}
	filter.From, filter.To = &from, &to
	return filter, nil
}

func integerQuery(c *gin.Context, key string, fallback int) (int, error) {
	value := c.Query(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%w: %s must be an integer", usecase.ErrInvalidInput, key)
	}
	return parsed, nil
}

func boolQuery(c *gin.Context, key string) (bool, error) {
	value := c.Query(key)
	if value == "" {
		return false, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%w: %s must be true or false", usecase.ErrInvalidInput, key)
	}
	return parsed, nil
}

func int64Query(c *gin.Context, key string, fallback int64) (int64, error) {
	value := c.Query(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %s must be an integer", usecase.ErrInvalidInput, key)
	}
	return parsed, nil
}

func floatQuery(c *gin.Context, key string, fallback float64) (float64, error) {
	value := c.Query(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %s must be a number", usecase.ErrInvalidInput, key)
	}
	return parsed, nil
}

type responseMeta struct {
	Count int `json:"count"`
}

type collectionResponse[T any] struct {
	Data []T          `json:"data"`
	Meta responseMeta `json:"meta"`
}

type apiError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestId,omitempty"`
}

type errorResponse struct {
	Error apiError `json:"error"`
}
