package http

import (
	"net/url"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

func NewRouter(handler *Handler, allowedOrigins []string) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), requestid.New())
	crossOrigin := cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{"GET", "OPTIONS"},
		AllowHeaders:     []string{"Accept", "Content-Type", "X-Request-ID"},
		ExposeHeaders:    []string{"X-Request-ID"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	})
	// Browsers send Origin on same-origin POSTs too, which the CORS middleware would reject
	// unless listed; CORS only concerns other origins, so same-origin requests skip it.
	router.Use(func(c *gin.Context) {
		if origin, err := url.Parse(c.GetHeader("Origin")); err == nil && origin.Host != "" && hostname(origin.Host) == hostname(c.Request.Host) {
			c.Next()
			return
		}
		crossOrigin(c)
	})

	router.GET("/healthz", handler.Health)
	router.GET("/readyz", handler.Ready)
	v1 := router.Group("/api/v1")
	{
		v1.GET("/transactions", handler.ListTransactions)
		v1.GET("/aggregations/h3", handler.AggregateH3)
		v1.GET("/aggregations/h3/cell", handler.CellDetail)
		v1.GET("/trends", handler.Trends)
		v1.GET("/summary", handler.Summary)
		v1.GET("/analysis", handler.Analysis)
		v1.GET("/districts/compare", handler.CompareDistricts)
		v1.GET("/investment/areas", handler.InvestmentAreas)
		v1.GET("/investment/workbench", handler.InvestmentWorkbench)
		v1.GET("/investment/liquidity", handler.InvestmentLiquidity)
		v1.GET("/rentals/districts", handler.RentalDistricts)
		v1.GET("/rentals/trend", handler.RentalTrend)
		v1.GET("/estimate", handler.Estimate)
		v1.GET("/estimate/building", handler.EstimateBuilding)
		v1.GET("/presales/projects", handler.PresaleProjects)
		v1.GET("/presales/project", handler.PresaleProject)
		v1.GET("/facilities", handler.Facilities)
		v1.GET("/facilities/nearby", handler.NearbyFacilities)
		v1.GET("/facilities/density", handler.FacilityDensity)
		v1.GET("/facilities/districts", handler.DistrictFacilities)
		v1.GET("/districts/boundaries", handler.DistrictBoundaries)
		v1.GET("/places", handler.SearchPlaces)
		v1.GET("/feed", handler.Feed)
		v1.GET("/export.csv", handler.ExportCSV)
		v1.GET("/regions", handler.Regions)
		v1.GET("/transit/lines", handler.TransitLines)
		v1.GET("/transit/stations", handler.TransitStations)
		v1.GET("/transit/stations/yoy", handler.StationYoYs)
		v1.GET("/transit/stations/ranking", handler.StationRanking)
		v1.GET("/transit/openings", handler.OpeningEffects)
		v1.GET("/transit/distance-curve", handler.DistanceCurve)
		v1.GET("/transit/stations/:id/trend", handler.StationTrend)
		v1.GET("/transit/stations/:id/rent", handler.StationRent)

		// The admin console is for allowed Tailscale users only; /admin/me says who you are.
		v1.GET("/admin/me", handler.AdminMe)
		admin := v1.Group("/admin", handler.requireAdmin)
		admin.GET("/data-status", handler.AdminDataStatus)
		admin.GET("/system", handler.AdminSystem)
		admin.GET("/jobs", handler.AdminJobs)
		admin.POST("/jobs", handler.requireSameOrigin, handler.AdminEnqueueJob)
		admin.GET("/openings", handler.AdminOpenings)
		admin.PUT("/openings", handler.requireSameOrigin, handler.AdminSaveOpening)
	}
	return router
}
