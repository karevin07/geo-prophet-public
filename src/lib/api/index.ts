import { HTTPTransactionAPI } from "./http-transaction-api";
import type { TransactionAPI } from "./types";

export type { InvestmentWorkbenchArea, InvestmentWorkbenchReport } from "./types";
export type { InvestmentLiquidityMonth, InvestmentLiquidityArea, InvestmentLiquidityReport } from "./types";

export function createTransactionAPI(): TransactionAPI {
  if (import.meta.env.VITE_DATA_SOURCE === "mock") return new LazyMockTransactionAPI();
  return new HTTPTransactionAPI(import.meta.env.VITE_API_BASE_URL ?? "");
}

class LazyMockTransactionAPI implements TransactionAPI {
  private readonly api: Promise<TransactionAPI> = import("./mock-transaction-api").then(({ MockTransactionAPI }) => new MockTransactionAPI());

  async list(...args: Parameters<TransactionAPI["list"]>) {
    return (await this.api).list(...args);
  }

  async aggregateH3(...args: Parameters<TransactionAPI["aggregateH3"]>) {
    return (await this.api).aggregateH3(...args);
  }

  async trends(...args: Parameters<TransactionAPI["trends"]>) {
    return (await this.api).trends(...args);
  }

  async summary(...args: Parameters<TransactionAPI["summary"]>) {
    return (await this.api).summary(...args);
  }

  async analysis(...args: Parameters<TransactionAPI["analysis"]>) {
    return (await this.api).analysis(...args);
  }

  async compareDistricts(...args: Parameters<TransactionAPI["compareDistricts"]>) {
    return (await this.api).compareDistricts(...args);
  }

  async investmentAreas(...args: Parameters<TransactionAPI["investmentAreas"]>) {
    return (await this.api).investmentAreas(...args);
  }

  async investmentWorkbench(...args: Parameters<TransactionAPI["investmentWorkbench"]>) {
    return (await this.api).investmentWorkbench(...args);
  }

  async investmentLiquidity(...args: Parameters<TransactionAPI["investmentLiquidity"]>) {
    return (await this.api).investmentLiquidity(...args);
  }

  async rentalDistricts(...args: Parameters<TransactionAPI["rentalDistricts"]>) {
    return (await this.api).rentalDistricts(...args);
  }

  async rentalTrend(...args: Parameters<TransactionAPI["rentalTrend"]>) {
    return (await this.api).rentalTrend(...args);
  }

  async facilities(...args: Parameters<TransactionAPI["facilities"]>) {
    return (await this.api).facilities(...args);
  }

  async nearbyFacilities(...args: Parameters<TransactionAPI["nearbyFacilities"]>) {
    return (await this.api).nearbyFacilities(...args);
  }

  async districtFacilities(...args: Parameters<TransactionAPI["districtFacilities"]>) {
    return (await this.api).districtFacilities(...args);
  }

  async facilityDensity(...args: Parameters<TransactionAPI["facilityDensity"]>) {
    return (await this.api).facilityDensity(...args);
  }

  async presaleProjects(...args: Parameters<TransactionAPI["presaleProjects"]>) {
    return (await this.api).presaleProjects(...args);
  }

  async presaleProject(...args: Parameters<TransactionAPI["presaleProject"]>) {
    return (await this.api).presaleProject(...args);
  }

  async estimate(...args: Parameters<TransactionAPI["estimate"]>) {
    return (await this.api).estimate(...args);
  }

  async estimateBuilding(...args: Parameters<TransactionAPI["estimateBuilding"]>) {
    return (await this.api).estimateBuilding(...args);
  }

  async stationRent(...args: Parameters<TransactionAPI["stationRent"]>) {
    return (await this.api).stationRent(...args);
  }

  async searchPlaces(...args: Parameters<TransactionAPI["searchPlaces"]>) {
    return (await this.api).searchPlaces(...args);
  }

  async feed(...args: Parameters<TransactionAPI["feed"]>) {
    return (await this.api).feed(...args);
  }

  async regions(...args: Parameters<TransactionAPI["regions"]>) {
    return (await this.api).regions(...args);
  }

  async cellDetail(...args: Parameters<TransactionAPI["cellDetail"]>) {
    return (await this.api).cellDetail(...args);
  }

  async districtBoundaries(...args: Parameters<TransactionAPI["districtBoundaries"]>) {
    return (await this.api).districtBoundaries(...args);
  }

  async transitLines(...args: Parameters<TransactionAPI["transitLines"]>) {
    return (await this.api).transitLines(...args);
  }

  async transitStations(...args: Parameters<TransactionAPI["transitStations"]>) {
    return (await this.api).transitStations(...args);
  }

  async stationYoYs(...args: Parameters<TransactionAPI["stationYoYs"]>) {
    return (await this.api).stationYoYs(...args);
  }

  async stationTrend(...args: Parameters<TransactionAPI["stationTrend"]>) {
    return (await this.api).stationTrend(...args);
  }

  async stationRanking(...args: Parameters<TransactionAPI["stationRanking"]>) {
    return (await this.api).stationRanking(...args);
  }

  async distanceCurve(...args: Parameters<TransactionAPI["distanceCurve"]>) {
    return (await this.api).distanceCurve(...args);
  }

  async openingEffects(...args: Parameters<TransactionAPI["openingEffects"]>) {
    return (await this.api).openingEffects(...args);
  }

  // Mock mode has no server to export from.
  exportUrl() {
    return "";
  }
}

export { BUILDING_TYPES, FEED_SORTS, STATION_RADII, buildingCategory } from "./types";
export type { AnalysisQuery, BBox, BreakdownAnalysis, BuildingType, CategoryAnalysis, CellDetail, DistrictBoundaries, DistrictComparison, DistrictQuery, DistrictRank, DistrictTrend, FeedPage, FeedQuery, FeedSort, H3Aggregate, InvestmentArea, InvestmentQuery, InvestmentReport, BuildingProfile, CityFacilities, Comparable, DistrictFacilities, ComparableLease, RentEstimate, Facility, FacilityCell, FacilityGroup, FacilityList, NearbyCategory, NearbyFacilities, PresaleProject, PresaleProjectDetail, PresaleProjects, PresaleUnit, ResaleComparison, Estimate, EstimateConfidence, EstimateQuery, RentalDistrict, RentalQuery, RentalReport, RentPoint, RentTrend, StationRent, StationRentRing, MarketAnalysis, MonthlyAnalysis, PriceBucket, Region, Release, MarketSummary, Place, SummaryWithYoY, YearOverYear, TransactionAPI, TransactionData, TransactionQuery, TrendPoint, StationLine, StationRadius, StationRing, StationTrend, StationYoYs, TransitLines, TransitStation, TransitSystem, CityDataCount, DataStatus, QualityAlert, QualityMetric, ImportDataset, ImportErrorKind, ImportRun, ImportRunState, SeasonCoverage, OpeningEffect, OpeningEffects, OpeningPoint, OpeningWindows, StationRank, StationRanking, DistanceBucket } from "./types";
