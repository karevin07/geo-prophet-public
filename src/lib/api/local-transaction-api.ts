import type { LocalSales } from "./local-sales";
import type { AnalysisQuery, BBox, BuildingProfile, CellDetail, CityFacilities, DistanceBucket, DistrictBoundaries, DistrictComparison, DistrictQuery, Estimate, FacilityCell, FacilityList, FeedPage, FeedQuery, H3Aggregate, InvestmentLiquidityReport, InvestmentQuery, InvestmentReport, InvestmentWorkbenchReport, MarketAnalysis, NearbyFacilities, OpeningEffects, Place, PresaleProjectDetail, PresaleProjects, Region, RentalQuery, RentalReport, RentTrend, StationRadius, StationRanking, StationRent, StationTrend, StationYoYs, SummaryWithYoY, TransactionAPI, TransactionData, TransactionQuery, TransitLines, TransitStation, TrendPoint } from "./types";

/**
 * Answers without an API: sales queries from LocalSales, everything else empty, so those
 * pages show their empty states. The mock and the online demo override what they have.
 */
export abstract class LocalTransactionAPI implements TransactionAPI {
  protected abstract sales(): Promise<LocalSales>;

  async list(query: TransactionQuery): Promise<TransactionData[]> {
    return (await this.sales()).list(query);
  }

  async aggregateH3(query: TransactionQuery, resolution: number): Promise<H3Aggregate[]> {
    return (await this.sales()).aggregateH3(query, resolution);
  }

  async trends(query: TransactionQuery): Promise<TrendPoint[]> {
    return (await this.sales()).trends(query);
  }

  async summary(query: TransactionQuery): Promise<SummaryWithYoY> {
    return (await this.sales()).summary(query);
  }

  async analysis(query: AnalysisQuery): Promise<MarketAnalysis> {
    return (await this.sales()).analysis(query);
  }

  async compareDistricts(query: DistrictQuery): Promise<DistrictComparison> {
    return (await this.sales()).compareDistricts(query);
  }

  async cellDetail(query: TransactionQuery, hex: string): Promise<CellDetail> {
    return (await this.sales()).cellDetail(query, hex);
  }

  async feed(query: FeedQuery, cursor?: string): Promise<FeedPage> {
    return (await this.sales()).feed(query, cursor);
  }

  async regions(): Promise<Region[]> {
    return (await this.sales()).regions();
  }

  async searchPlaces(text: string): Promise<Place[]> {
    return (await this.sales()).searchPlaces(text);
  }

  // There is no export without the API.
  exportUrl(_query: FeedQuery): string {
    return "";
  }

  async districtBoundaries(_city: string): Promise<DistrictBoundaries> {
    return { type: "FeatureCollection", features: [] };
  }

  async investmentAreas(query: InvestmentQuery): Promise<InvestmentReport> {
    return { cityCode: query.city, through: "", currentFrom: "", previousFrom: "", minimumSample: 30, cityCurrentCount: 0, cityPreviousCount: 0,
      cityMedianPricePerPing: null, cityVolumeChange: null, cityPriceChange: null, areas: [] };
  }

  async investmentWorkbench(query: InvestmentQuery): Promise<InvestmentWorkbenchReport> {
    return { cityCode: query.city, through: "", currentFrom: "", previousFrom: "", minimumSales: 30, minimumRentals: 20, areas: [] };
  }

  async investmentLiquidity(query: InvestmentQuery): Promise<InvestmentLiquidityReport> {
    return { cityCode: query.city, through: "", currentFrom: "", previousFrom: "", minimumSample: 30,
      currentCoverageComplete: false, previousCoverageComplete: false, incompleteMonths: [], areas: [] };
  }

  async rentalDistricts(query: RentalQuery): Promise<RentalReport> {
    const empty = { district: "", rentals: 0, medianRentPerPing: null, medianMonthlyRent: null, sales: 0, medianPricePerPing: null, grossYield: null };
    return { cityCode: query.city, from: "", through: "", minimumSample: 20, includeSocialHousing: query.includeSocial, city: empty, districts: [] };
  }

  async rentalTrend(query: RentalQuery, district: string): Promise<RentTrend> {
    return { cityCode: query.city, district, through: "", minimumSample: 10, city: [], districtPoints: [] };
  }

  async facilities(_bbox: BBox, _categories: string[]): Promise<FacilityList> {
    return { facilities: [], truncated: false };
  }

  async nearbyFacilities(): Promise<NearbyFacilities> {
    return { radiusM: 500, searchM: 2000, categories: [] };
  }

  async districtFacilities(city: string): Promise<CityFacilities> {
    return { cityCode: city, city: { district: "", areaKm2: 0, counts: {} }, districts: [] };
  }

  async facilityDensity(): Promise<FacilityCell[]> {
    return [];
  }

  async presaleProjects(query: { city: string; months: number }): Promise<PresaleProjects> {
    return { cityCode: query.city, from: "", projects: [] };
  }

  async presaleProject(key: string): Promise<PresaleProjectDetail> {
    throw new Error(`no presale project ${key}`);
  }

  // Estimates need door numbers and comparable-sale searches, so they are always insufficient.
  async estimate(): Promise<Estimate> {
    return {
      cityCode: "a", district: "", asOf: "", sufficient: false, minimumSample: 5, pricePerPing: null, lowPricePerPing: null,
      highPricePerPing: null, buildingPrice: null, parkingPrice: null, totalPrice: null, lowPrice: null, highPrice: null,
      confidence: "", spread: 0, backtestMedianError: 0, rangeLow: 0, rangeHigh: 0, comparables: [], comparableSales: 0, sameBuildingSales: 0, maxDistanceM: 0, rent: null,
    };
  }

  async estimateBuilding(address: string): Promise<BuildingProfile> {
    return { address, sales: 0, category: "", completedYear: null, totalFloors: null, lastSale: "" };
  }

  // Rail stations come from the TDX import, which needs an API key.
  async stationRent(stationId: string): Promise<StationRent> {
    return { stationId, cityCode: "a", district: "", from: "", through: "", minimumSample: 20, districtMedianRentPerPing: null, rings: [] };
  }

  async transitLines(): Promise<TransitLines> {
    return { type: "FeatureCollection", features: [] };
  }

  async transitStations(): Promise<TransitStation[]> {
    return [];
  }

  async stationYoYs(_query: TransactionQuery, radius: StationRadius): Promise<StationYoYs> {
    return { radius, yoyThrough: "", stations: [] };
  }

  async stationTrend(): Promise<StationTrend> {
    throw new Error("no rail stations without the API");
  }

  async stationRanking(_query: TransactionQuery, radius: StationRadius): Promise<StationRanking> {
    return { radius, yoyThrough: "", stations: [] };
  }

  async distanceCurve(): Promise<DistanceBucket[]> {
    return [];
  }

  async openingEffects(_query: TransactionQuery, radius: StationRadius): Promise<OpeningEffects> {
    return { radius, events: [] };
  }
}
