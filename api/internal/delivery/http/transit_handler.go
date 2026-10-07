package http

import (
	"net/http"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/gin-gonic/gin"
)

// TransitLines serves rail routes as GeoJSON. They change only when stations are
// re-imported, so browsers may cache them for an hour.
func (handler *Handler) TransitLines(c *gin.Context) {
	lines, err := handler.transit.Lines(c.Request.Context())
	if err != nil {
		handler.writeError(c, err)
		return
	}
	features := make([]gin.H, 0, len(lines))
	for _, line := range lines {
		features = append(features, gin.H{"type": "Feature", "properties": line, "geometry": line.Geometry})
	}
	c.Header("Cache-Control", "public, max-age=3600")
	c.JSON(http.StatusOK, gin.H{"type": "FeatureCollection", "features": features})
}

func (handler *Handler) TransitStations(c *gin.Context) {
	stations, err := handler.transit.Stations(c.Request.Context(), c.Query("city"))
	if err != nil {
		handler.writeError(c, err)
		return
	}
	c.Header("Cache-Control", "public, max-age=3600")
	c.JSON(http.StatusOK, collectionResponse[domain.TransitStation]{Data: stations, Meta: responseMeta{Count: len(stations)}})
}

// StationTrend describes sales around one station: figures for the date window (by
// default the last 12 months) and a trend over all years.
func (handler *Handler) StationTrend(c *gin.Context) {
	filter, err := handler.parseFilter(c)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	trend, err := handler.transit.StationTrend(c.Request.Context(), c.Param("id"), filter)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": trend})
}

// StationYoYs gives every station's YoY within one radius, for colouring the map.
func (handler *Handler) StationYoYs(c *gin.Context) {
	filter, err := handler.parseFilter(c)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	radius, err := integerQuery(c, "radius", 500)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	result, err := handler.transit.StationYoYs(c.Request.Context(), filter, radius)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

// StationRanking gives every station's figures within one radius for the date window.
func (handler *Handler) StationRanking(c *gin.Context) {
	filter, err := handler.parseFilter(c)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	radius, err := integerQuery(c, "radius", 500)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	result, err := handler.transit.StationRanking(c.Request.Context(), filter, radius)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

// OpeningEffects compares price changes around each line opening with the district.
func (handler *Handler) OpeningEffects(c *gin.Context) {
	filter, err := handler.parseFilter(c)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	radius, err := integerQuery(c, "radius", 500)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	result, err := handler.transit.OpeningEffects(c.Request.Context(), filter, radius)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

// DistanceCurve gives price per ping by distance to the nearest station, by city.
func (handler *Handler) DistanceCurve(c *gin.Context) {
	filter, err := handler.parseFilter(c)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	buckets, err := handler.transit.DistanceCurve(c.Request.Context(), filter)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, collectionResponse[domain.DistanceBucket]{Data: buckets, Meta: responseMeta{Count: len(buckets)}})
}
