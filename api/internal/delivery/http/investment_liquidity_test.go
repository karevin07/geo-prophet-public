package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	delivery "github.com/geo-prophet/geo-prophet/api/internal/delivery/http"
	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/geo-prophet/geo-prophet/api/internal/usecase"
	"github.com/gin-gonic/gin"
)

type liquidityHTTPRepository struct{ query domain.InvestmentQuery }

func (*liquidityHTTPRepository) ImportedSalesByCity(context.Context, string) ([]string, []string, error) {
	return []string{"113S1", "113S2", "113S3", "113S4", "114S1", "114S2", "114S3", "114S4", "115S1", "115S2"}, nil, nil
}

func (stub *liquidityHTTPRepository) InvestmentMonthlyCounts(_ context.Context, query domain.InvestmentQuery, _, _ time.Time) ([]domain.InvestmentMonthlyCount, error) {
	stub.query = query
	return []domain.InvestmentMonthlyCount{{District: "板橋區", Month: "2025-03", Count: 35}, {District: "板橋區", Month: "2024-03", Count: 30}}, nil
}

func TestInvestmentLiquidityHTTPContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &liquidityHTTPRepository{}
	configured := services(&repositoryStub{})
	configured.Liquidity = usecase.NewInvestmentLiquidityService(stub)
	router := delivery.NewRouter(delivery.NewHandler(configured), []string{"http://localhost:3000"})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/investment/liquidity?city=f&type=公寓&min_area=15&max_area=30&min_age=5&max_age=30", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status %d: %s", response.Code, response.Body.String())
	}
	var body struct {
		Data domain.InvestmentLiquidityReport `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if stub.query.CityCode != "f" || stub.query.Category != "公寓" || stub.query.MaxArea != 30 || stub.query.MinAge != 5 || len(body.Data.Areas) != 1 || len(body.Data.Areas[0].Monthly) != 24 || !body.Data.CurrentCoverageComplete {
		t.Fatalf("wrong cohort or contract: %+v, %+v", stub.query, body.Data)
	}
	for _, target := range []string{"/api/v1/investment/liquidity?city=z", "/api/v1/investment/liquidity?min_age=40&max_age=5", "/api/v1/investment/liquidity?min_area=bad"} {
		response = httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s returned %d", target, response.Code)
		}
	}
	missing := serve(t, &repositoryStub{}, "/api/v1/investment/liquidity")
	if missing.Code != http.StatusServiceUnavailable {
		t.Fatalf("without DB returned %d", missing.Code)
	}
}
