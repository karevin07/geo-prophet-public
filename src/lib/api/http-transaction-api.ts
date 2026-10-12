import type { BBox, DistanceBucket, OpeningEffects, StationRanking, CellDetail, DistrictBoundaries, StationRadius, StationTrend, StationYoYs, TransitLines, TransitStation, AnalysisQuery, DistrictComparison, DistrictQuery, FeedPage, FeedQuery, H3Aggregate, InvestmentQuery, InvestmentReport, InvestmentWorkbenchReport, MarketAnalysis, BuildingProfile, CityFacilities, Estimate, EstimateQuery, FacilityCell, FacilityList, NearbyFacilities, PresaleProjectDetail, PresaleProjects, RentalQuery, RentalReport, RentTrend, StationRent, Place, Region, SummaryWithYoY, TransactionAPI, TransactionData, TransactionQuery, TrendPoint } from "./types";

import type { InvestmentLiquidityReport } from "./types";

interface CollectionResponse<T> {
  data: T[];
  meta: { count: number };
}

function queryParams(query: TransactionQuery): URLSearchParams {
  const params = new URLSearchParams({
    min_price: String(query.priceRange[0]),
    max_price: String(query.priceRange[1]),
    min_area: String(query.areaRange[0]),
    max_area: String(query.areaRange[1]),
    types: query.types.join(","),
  });

  if (query.bbox) params.set("bbox", query.bbox.join(","));
  if (query.month) {
    params.set("from", query.month);
    params.set("to", query.month);
  }
  if (query.includeSpecial) params.set("include_special", "true");
  if (query.excludeNewBuildings) params.set("exclude_new", "true");
  return params;
}

function feedParams(query: FeedQuery): URLSearchParams {
  const params = queryParams(query);
  if (query.from) params.set("from", query.from);
  if (query.to) params.set("to", query.to);
  if (query.city) params.set("city", query.city);
  if (query.district) params.set("district", query.district);
  if (query.latestOnly) params.set("latest", "true");
  params.set("sort", query.sort);
  return params;
}

function analysisParams(query: AnalysisQuery): URLSearchParams {
  const params = queryParams(query);
  params.set("from", query.from);
  params.set("to", query.to);
  return params;
}

function investmentParams(query: InvestmentQuery): URLSearchParams {
  return new URLSearchParams({
    city: query.city, type: query.type, min_area: String(query.minArea), max_area: String(query.maxArea),
    min_age: String(query.minAge), max_age: String(query.maxAge),
  });
}

function rentalParams(query: RentalQuery): URLSearchParams {
  const params = new URLSearchParams({ city: query.city });
  if (query.type) params.set("type", query.type);
  if (query.minArea > 0) params.set("min_area", String(query.minArea));
  if (query.maxArea > 0) params.set("max_area", String(query.maxArea));
  if (query.includeSocial) params.set("include_social", "true");
  return params;
}

function districtParams(query: DistrictQuery): URLSearchParams {
  const params = queryParams(query);
  params.set("city", query.city);
  params.set("districts", query.districts.join(","));
  params.set("from", query.from);
  params.set("to", query.to);
  return params;
}

/** Reads a JSON response; the online demo replaces it to read saved responses instead. */
export type FetchJSON = <T>(url: string, signal?: AbortSignal) => Promise<T>;

async function getJSON<T>(url: string, signal?: AbortSignal): Promise<T> {
  const response = await fetch(url, { signal, headers: { Accept: "application/json" } });
  if (!response.ok) {
    const body = await response.json().catch(() => null) as { error?: { message?: string } } | null;
    throw new Error(body?.error?.message ?? `API request failed (${response.status})`);
  }
  return response.json() as Promise<T>;
}

export class HTTPTransactionAPI implements TransactionAPI {
  constructor(private readonly baseURL = "", private readonly get: FetchJSON = getJSON) {}

  async list(query: TransactionQuery, signal?: AbortSignal): Promise<TransactionData[]> {
    const params = queryParams(query);
    params.set("limit", "2000");
    return (await this.get<CollectionResponse<TransactionData>>(`${this.baseURL}/api/v1/transactions?${params}`, signal)).data;
  }

  async aggregateH3(query: TransactionQuery, resolution: number, signal?: AbortSignal): Promise<H3Aggregate[]> {
    const params = queryParams(query);
    params.set("resolution", String(resolution));
    return (await this.get<CollectionResponse<H3Aggregate>>(`${this.baseURL}/api/v1/aggregations/h3?${params}`, signal)).data;
  }

  async trends(query: TransactionQuery, signal?: AbortSignal): Promise<TrendPoint[]> {
    return (await this.get<CollectionResponse<TrendPoint>>(`${this.baseURL}/api/v1/trends?${queryParams(query)}`, signal)).data;
  }

  async summary(query: TransactionQuery, signal?: AbortSignal): Promise<SummaryWithYoY> {
    return (await this.get<{ data: SummaryWithYoY }>(`${this.baseURL}/api/v1/summary?${queryParams(query)}`, signal)).data;
  }

  async analysis(query: AnalysisQuery, signal?: AbortSignal): Promise<MarketAnalysis> {
    return (await this.get<{ data: MarketAnalysis }>(`${this.baseURL}/api/v1/analysis?${analysisParams(query)}`, signal)).data;
  }

  async compareDistricts(query: DistrictQuery, signal?: AbortSignal): Promise<DistrictComparison> {
    return (await this.get<{ data: DistrictComparison }>(`${this.baseURL}/api/v1/districts/compare?${districtParams(query)}`, signal)).data;
  }

  async investmentAreas(query: InvestmentQuery, signal?: AbortSignal): Promise<InvestmentReport> {
    const params = investmentParams(query);
    return (await this.get<{ data: InvestmentReport }>(`${this.baseURL}/api/v1/investment/areas?${params}`, signal)).data;
  }

  async investmentWorkbench(query: InvestmentQuery, signal?: AbortSignal): Promise<InvestmentWorkbenchReport> {
    const params = investmentParams(query);
    return (await this.get<{ data: InvestmentWorkbenchReport }>(`${this.baseURL}/api/v1/investment/workbench?${params}`, signal)).data;
  }

  async investmentLiquidity(query: InvestmentQuery, signal?: AbortSignal): Promise<InvestmentLiquidityReport> {
    return (await this.get<{ data: InvestmentLiquidityReport }>(`${this.baseURL}/api/v1/investment/liquidity?${investmentParams(query)}`, signal)).data;
  }

  async rentalDistricts(query: RentalQuery, signal?: AbortSignal): Promise<RentalReport> {
    return (await this.get<{ data: RentalReport }>(`${this.baseURL}/api/v1/rentals/districts?${rentalParams(query)}`, signal)).data;
  }

  async rentalTrend(query: RentalQuery, district: string, signal?: AbortSignal): Promise<RentTrend> {
    const params = rentalParams(query);
    if (district) params.set("district", district);
    return (await this.get<{ data: RentTrend }>(`${this.baseURL}/api/v1/rentals/trend?${params}`, signal)).data;
  }

  async facilities(bbox: BBox, categories: string[], signal?: AbortSignal): Promise<FacilityList> {
    const params = new URLSearchParams({ bbox: bbox.join(","), categories: categories.join(",") });
    return (await this.get<{ data: FacilityList }>(`${this.baseURL}/api/v1/facilities?${params}`, signal)).data;
  }

  async nearbyFacilities(lat: number, lng: number, radius: number, signal?: AbortSignal): Promise<NearbyFacilities> {
    const params = new URLSearchParams({ lat: lat.toFixed(6), lng: lng.toFixed(6), radius: String(radius) });
    return (await this.get<{ data: NearbyFacilities }>(`${this.baseURL}/api/v1/facilities/nearby?${params}`, signal)).data;
  }

  async districtFacilities(city: string, signal?: AbortSignal): Promise<CityFacilities> {
    return (await this.get<{ data: CityFacilities }>(`${this.baseURL}/api/v1/facilities/districts?${new URLSearchParams({ city })}`, signal)).data;
  }

  async facilityDensity(bbox: BBox, resolution: 8 | 9, categories: string[], signal?: AbortSignal): Promise<FacilityCell[]> {
    const params = new URLSearchParams({ bbox: bbox.join(","), res: String(resolution), categories: categories.join(",") });
    return (await this.get<{ data: FacilityCell[] }>(`${this.baseURL}/api/v1/facilities/density?${params}`, signal)).data;
  }

  async presaleProjects(query: { city: string; district?: string; months: number; near?: { lat: number; lng: number; radius: number }; bbox?: BBox }, signal?: AbortSignal): Promise<PresaleProjects> {
    const params = new URLSearchParams({ city: query.city, months: String(query.months) });
    if (query.district) params.set("district", query.district);
    if (query.near) {
      params.set("lat", String(query.near.lat));
      params.set("lng", String(query.near.lng));
      params.set("radius", String(query.near.radius));
    }
    if (query.bbox) params.set("bbox", query.bbox.join(","));
    return (await this.get<{ data: PresaleProjects }>(`${this.baseURL}/api/v1/presales/projects?${params}`, signal)).data;
  }

  async presaleProject(key: string, signal?: AbortSignal): Promise<PresaleProjectDetail> {
    return (await this.get<{ data: PresaleProjectDetail }>(`${this.baseURL}/api/v1/presales/project?${new URLSearchParams({ key })}`, signal)).data;
  }

  async estimate(query: EstimateQuery, signal?: AbortSignal): Promise<Estimate> {
    const params = new URLSearchParams({
      address: query.address, lat: String(query.lat), lng: String(query.lng), type: query.type, area: String(query.area),
    });
    if (query.age !== null) params.set("age", String(query.age));
    if (query.floor > 0) params.set("floor", String(query.floor));
    if (query.totalFloors > 0) params.set("total_floors", String(query.totalFloors));
    if (query.parking) params.set("parking", "true");
    return (await this.get<{ data: Estimate }>(`${this.baseURL}/api/v1/estimate?${params}`, signal)).data;
  }

  async estimateBuilding(address: string, lat: number, lng: number, signal?: AbortSignal): Promise<BuildingProfile> {
    const params = new URLSearchParams({ address, lat: String(lat), lng: String(lng) });
    return (await this.get<{ data: BuildingProfile }>(`${this.baseURL}/api/v1/estimate/building?${params}`, signal)).data;
  }

  async stationRent(stationId: string, signal?: AbortSignal): Promise<StationRent> {
    return (await this.get<{ data: StationRent }>(`${this.baseURL}/api/v1/transit/stations/${encodeURIComponent(stationId)}/rent`, signal)).data;
  }

  async feed(query: FeedQuery, cursor?: string, signal?: AbortSignal): Promise<FeedPage> {
    const params = feedParams(query);
    if (cursor) params.set("cursor", cursor);
    return this.get<FeedPage>(`${this.baseURL}/api/v1/feed?${params}`, signal);
  }

  async regions(signal?: AbortSignal): Promise<Region[]> {
    return (await this.get<CollectionResponse<Region>>(`${this.baseURL}/api/v1/regions`, signal)).data;
  }

  async cellDetail(query: TransactionQuery, hex: string, signal?: AbortSignal): Promise<CellDetail> {
    const params = queryParams(query);
    params.set("hex", hex);
    return (await this.get<{ data: CellDetail }>(`${this.baseURL}/api/v1/aggregations/h3/cell?${params}`, signal)).data;
  }

  async districtBoundaries(city: string, signal?: AbortSignal): Promise<DistrictBoundaries> {
    return this.get<DistrictBoundaries>(`${this.baseURL}/api/v1/districts/boundaries?${new URLSearchParams({ city })}`, signal);
  }

  async transitLines(signal?: AbortSignal): Promise<TransitLines> {
    return this.get<TransitLines>(`${this.baseURL}/api/v1/transit/lines`, signal);
  }

  async transitStations(signal?: AbortSignal): Promise<TransitStation[]> {
    return (await this.get<CollectionResponse<TransitStation>>(`${this.baseURL}/api/v1/transit/stations`, signal)).data;
  }

  async stationYoYs(query: TransactionQuery, radius: StationRadius, signal?: AbortSignal): Promise<StationYoYs> {
    const params = queryParams({ ...query, bbox: undefined, month: undefined });
    params.set("radius", String(radius));
    return (await this.get<{ data: StationYoYs }>(`${this.baseURL}/api/v1/transit/stations/yoy?${params}`, signal)).data;
  }

  async stationTrend(query: TransactionQuery, stationId: string, signal?: AbortSignal): Promise<StationTrend> {
    const params = queryParams({ ...query, bbox: undefined });
    return (await this.get<{ data: StationTrend }>(`${this.baseURL}/api/v1/transit/stations/${encodeURIComponent(stationId)}/trend?${params}`, signal)).data;
  }

  async stationRanking(query: TransactionQuery, radius: StationRadius, from?: string, signal?: AbortSignal): Promise<StationRanking> {
    const params = queryParams({ ...query, bbox: undefined, month: undefined });
    params.set("radius", String(radius));
    if (from) params.set("from", from);
    return (await this.get<{ data: StationRanking }>(`${this.baseURL}/api/v1/transit/stations/ranking?${params}`, signal)).data;
  }

  async distanceCurve(query: TransactionQuery, from?: string, signal?: AbortSignal): Promise<DistanceBucket[]> {
    const params = queryParams({ ...query, bbox: undefined, month: undefined });
    if (from) params.set("from", from);
    return (await this.get<CollectionResponse<DistanceBucket>>(`${this.baseURL}/api/v1/transit/distance-curve?${params}`, signal)).data;
  }

  async openingEffects(query: TransactionQuery, radius: StationRadius, signal?: AbortSignal): Promise<OpeningEffects> {
    const params = queryParams({ ...query, bbox: undefined, month: undefined });
    params.set("radius", String(radius));
    return (await this.get<{ data: OpeningEffects }>(`${this.baseURL}/api/v1/transit/openings?${params}`, signal)).data;
  }

  exportUrl(query: FeedQuery): string {
    return `${this.baseURL}/api/v1/export.csv?${feedParams(query)}`;
  }

  async searchPlaces(text: string, signal?: AbortSignal): Promise<Place[]> {
    const params = new URLSearchParams({ q: text });
    return (await this.get<CollectionResponse<Place>>(`${this.baseURL}/api/v1/places?${params}`, signal)).data;
  }
}
