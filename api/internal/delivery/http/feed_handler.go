package http

import (
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/geo-prophet/geo-prophet/api/internal/usecase"
	"github.com/gin-gonic/gin"
)

type feedMeta struct {
	Total         *int            `json:"total,omitempty"`
	NewCount      *int            `json:"newCount,omitempty"`
	NextCursor    string          `json:"nextCursor,omitempty"`
	LatestRelease *domain.Release `json:"latestRelease"`
}

func (handler *Handler) Feed(c *gin.Context) {
	query, err := handler.parseFeedQuery(c)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	page, err := handler.feed.Page(c.Request.Context(), query)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	meta := feedMeta{Total: page.Total, NewCount: page.NewCount, LatestRelease: page.LatestRelease}
	if page.NextCursor != nil {
		encoded, _ := json.Marshal(page.NextCursor)
		meta.NextCursor = base64.RawURLEncoding.EncodeToString(encoded)
	}
	c.JSON(http.StatusOK, gin.H{"data": page.Transactions, "meta": meta})
}

func (handler *Handler) Regions(c *gin.Context) {
	regions := usecase.Regions()
	c.JSON(http.StatusOK, collectionResponse[domain.Region]{Data: regions, Meta: responseMeta{Count: len(regions)}})
}

// csvHeader lists the export columns; Excel needs the UTF-8 BOM written before it.
var csvHeader = []string{
	"編號", "縣市代碼", "行政區", "地址", "成交日", "交易標的", "建物型態", "主要用途",
	"總價(元)", "車位價(元)", "建物價(元)", "建物坪數(不含車位)", "單價(元/坪)",
	"房", "廳", "衛", "移轉層次", "總樓層", "完工日", "特殊交易原因", "備註", "季別",
	"緯度", "經度", "定位精度", "本期新增",
}

func (handler *Handler) ExportCSV(c *gin.Context) {
	query, err := handler.parseFeedQuery(c)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	filename := fmt.Sprintf("geoprophet-transactions-%s.csv", handler.now().Format("20060102"))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Status(http.StatusOK)

	_, _ = c.Writer.WriteString("\ufeff")
	writer := csv.NewWriter(c.Writer)
	_ = writer.Write(csvHeader)
	err = handler.feed.Export(c.Request.Context(), query, func(transaction domain.Transaction) error {
		return writer.Write(csvRow(transaction))
	})
	writer.Flush()
	if err != nil {
		// Headers are already sent, so the truncated file is the only signal left.
		_ = c.Error(err)
	}
}

func csvRow(transaction domain.Transaction) []string {
	completed, totalFloors := "", ""
	if transaction.CompletedAt != nil {
		completed = transaction.CompletedAt.Format("2006-01-02")
	}
	if transaction.TotalFloors > 0 {
		totalFloors = strconv.Itoa(transaction.TotalFloors)
	}
	isNew := ""
	if transaction.IsNew {
		isNew = "Y"
	}
	return []string{
		transaction.ID, transaction.CityCode, transaction.District, transaction.Address,
		transaction.TransactedAt.Format("2006-01-02"), transaction.TransactionType, transaction.BuildingType, transaction.MainUse,
		strconv.FormatInt(transaction.TotalPrice, 10), strconv.FormatInt(transaction.ParkingPrice, 10),
		strconv.FormatInt(transaction.BuildingPrice, 10), strconv.FormatFloat(transaction.BuildingAreaPing, 'f', 2, 64),
		strconv.FormatFloat(transaction.PricePerPing, 'f', 0, 64),
		strconv.Itoa(transaction.Rooms), strconv.Itoa(transaction.Halls), strconv.Itoa(transaction.Baths),
		transaction.FloorLabel, totalFloors, completed, strings.Join(transaction.SpecialReasons, ";"), transaction.Remarks,
		transaction.Season, strconv.FormatFloat(transaction.Lat, 'f', 6, 64), strconv.FormatFloat(transaction.Lng, 'f', 6, 64),
		transaction.GeocodePrecision, isNew,
	}
}

func (handler *Handler) parseFeedQuery(c *gin.Context) (domain.FeedQuery, error) {
	filter, err := handler.parseFilter(c)
	if err != nil {
		return domain.FeedQuery{}, err
	}
	latest, err := boolQuery(c, "latest")
	if err != nil {
		return domain.FeedQuery{}, err
	}
	query := domain.FeedQuery{
		Filter:            filter,
		CityCode:          c.Query("city"),
		District:          c.Query("district"),
		LatestReleaseOnly: latest,
		Sort:              domain.FeedSort(c.Query("sort")),
	}
	if encoded := c.Query("cursor"); encoded != "" {
		var cursor domain.FeedCursor
		raw, decodeErr := base64.RawURLEncoding.DecodeString(encoded)
		if decodeErr != nil || json.Unmarshal(raw, &cursor) != nil || cursor.ID == "" {
			return domain.FeedQuery{}, fmt.Errorf("%w: malformed cursor", usecase.ErrInvalidInput)
		}
		if query.Sort == "" || strings.HasPrefix(string(query.Sort), "date") {
			if _, parseErr := time.Parse("2006-01-02", cursor.Value); parseErr != nil {
				return domain.FeedQuery{}, fmt.Errorf("%w: malformed cursor", usecase.ErrInvalidInput)
			}
		} else if _, parseErr := strconv.ParseFloat(cursor.Value, 64); parseErr != nil {
			return domain.FeedQuery{}, fmt.Errorf("%w: malformed cursor", usecase.ErrInvalidInput)
		}
		query.Cursor = &cursor
	}
	return query, nil
}
