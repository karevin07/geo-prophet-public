import { HTTPTransactionAPI } from "./http-transaction-api";
import type { TransactionAPI } from "./types";

export type { InvestmentWorkbenchArea, InvestmentWorkbenchReport } from "./types";
export type { InvestmentLiquidityMonth, InvestmentLiquidityArea, InvestmentLiquidityReport } from "./types";
export type { Community, CommunityDetail, CommunityRanking, CommunityRef, CommunityYear, NearbyCommunity, RankedCommunity } from "./types";

export function createTransactionAPI(): TransactionAPI {
  if (import.meta.env.VITE_DATA_SOURCE === "mock") return lazyAPI(() => import("./mock-transaction-api").then(({ MockTransactionAPI }) => new MockTransactionAPI()));
  if (import.meta.env.VITE_DATA_SOURCE === "snapshot") {
    return lazyAPI(() => import("./snapshot-transaction-api").then(({ SnapshotTransactionAPI }) => new SnapshotTransactionAPI(`${import.meta.env.BASE_URL}demo-data/`)));
  }
  return new HTTPTransactionAPI(import.meta.env.VITE_API_BASE_URL ?? "");
}

/**
 * Loads an implementation in its own chunk on first use, keeping the mock and the demo's
 * engine out of the main bundle. Only exportUrl is synchronous, and neither offers export.
 */
function lazyAPI(load: () => Promise<TransactionAPI>): TransactionAPI {
  let api: Promise<TransactionAPI> | undefined;
  return new Proxy({} as TransactionAPI, {
    get(_target, name: keyof TransactionAPI | "then") {
      if (name === "then") return undefined; // not a promise
      if (name === "exportUrl") return () => "";
      return async (...args: unknown[]) => {
        const loaded = await (api ??= load());
        return (loaded[name] as (...args: unknown[]) => unknown).apply(loaded, args);
      };
    },
  });
}

export { BUILDING_TYPES, FEED_SORTS, STATION_RADII, buildingCategory } from "./types";
export type { AnalysisQuery, BBox, BreakdownAnalysis, BuildingType, CategoryAnalysis, CellDetail, DistrictBoundaries, DistrictComparison, DistrictQuery, DistrictRank, DistrictTrend, FeedPage, FeedQuery, FeedSort, H3Aggregate, InvestmentArea, InvestmentQuery, InvestmentReport, BuildingProfile, CityFacilities, Comparable, DistrictFacilities, ComparableLease, RentEstimate, Facility, FacilityCell, FacilityGroup, FacilityList, NearbyCategory, NearbyFacilities, PresaleProject, PresaleProjectDetail, PresaleProjects, PresaleUnit, ResaleComparison, Estimate, EstimateConfidence, EstimateQuery, RentalDistrict, RentalQuery, RentalReport, RentCell, RentLayer, RentPoint, RentTrend, StationRent, StationRentRing, MarketAnalysis, MonthlyAnalysis, PriceBucket, Region, Release, MarketSummary, Place, SummaryWithYoY, YearOverYear, TransactionAPI, TransactionData, TransactionQuery, TrendPoint, StationLine, StationRadius, StationRing, StationTrend, StationYoYs, TransitLines, TransitStation, TransitSystem, CityDataCount, DataStatus, QualityAlert, QualityMetric, ImportDataset, ImportErrorKind, ImportRun, ImportRunState, SeasonCoverage, OpeningEffect, OpeningEffects, OpeningPoint, OpeningWindows, StationRank, StationRanking, DistanceBucket } from "./types";
