/** A completed sale from the MOI actual price registration (實價登錄). */
export interface TransactionData {
  id: string;
  cityCode: string;
  district: string;
  address: string;
  transactionType: string;
  buildingType: string;
  mainUse: string;
  totalPrice: number;
  parkingPrice: number;
  buildingPrice: number;
  buildingAreaPing: number;
  /** Price per ping excluding parking. */
  pricePerPing: number;
  rooms: number;
  halls: number;
  baths: number;
  floorLabel: string;
  totalFloors?: number;
  completedAt?: string;
  transactedAt: string;
  remarks?: string;
  specialReasons?: string[];
  season: string;
  lat: number;
  lng: number;
  geocodePrecision: "address" | "nearby" | "road" | "district";
  /** First published in the latest 10-day release. */
  isNew?: boolean;
}

export interface H3Aggregate {
  hex: string;
  lat: number;
  lng: number;
  count: number;
  /** Average total price. */
  price: number;
  avgArea: number;
  avgPricePerPing: number;
}

export interface TrendPoint {
  month: string;
  avgPricePerPing: number;
  count: number;
  /** Sales in this month may still be registered and published. */
  provisional: boolean;
}

/** Categories of 建物型態; 其他 matches every type not listed before it. */
export const BUILDING_TYPES = ["住宅大樓", "華廈", "公寓", "透天厝", "其他"] as const;
export type BuildingType = (typeof BUILDING_TYPES)[number];

export type BBox = [west: number, south: number, east: number, north: number];

export interface MarketSummary {
  count: number;
  /** Area-weighted price per ping, parking excluded. */
  avgPricePerPing: number;
  medianTotalPrice: number;
}

/** Relative changes against the same window a year earlier (0.041 = +4.1%); null when either period has too few sales. */
export interface YearOverYear {
  basis: string;
  /** Last month (YYYY-MM) of the compared window; stops at the last fully registered month. */
  through: string;
  count: number | null;
  avgPricePerPing: number | null;
  medianTotalPrice: number | null;
}

export interface SummaryWithYoY extends MarketSummary {
  yoy: YearOverYear | null;
}

export interface MonthlyAnalysis {
  month: string;
  count: number;
  avgPricePerPing: number;
  medianPricePerPing: number;
  medianTotalPrice: number;
  avgArea: number;
  provisional: boolean;
}

export interface CategoryAnalysis {
  category: BuildingType;
  count: number;
  avgPricePerPing: number;
  medianPricePerPing: number;
  medianTotalPrice: number;
}

export interface BreakdownAnalysis {
  key: string;
  count: number;
  avgPricePerPing: number;
  medianPricePerPing: number;
  medianTotalPrice: number;
}

export interface PriceBucket {
  minPricePerPing: number;
  maxPricePerPing: number;
  count: number;
}

export interface MarketAnalysis {
  monthly: MonthlyAnalysis[];
  categories: CategoryAnalysis[];
  ageGroups: BreakdownAnalysis[];
  areaGroups: BreakdownAnalysis[];
  distribution: PriceBucket[];
}

export interface DistrictQuery extends TransactionQuery {
  city: string;
  districts: string[];
  from: string;
  to: string;
}

export interface DistrictRank {
  district: string;
  count: number;
  avgPricePerPing: number;
  medianPricePerPing: number;
  medianTotalPrice: number;
  priceYoY: number | null;
}

export interface DistrictTrend {
  district: string;
  month: string;
  count: number;
  avgPricePerPing: number;
  provisional: boolean;
}

export interface DistrictComparison {
  cityCode: string;
  yoyThrough: string;
  ranking: DistrictRank[];
  trends: DistrictTrend[];
}

export interface RentalQuery {
  city: string;
  /** "" is every residential type. */
  type: string;
  minArea: number;
  /** 0 is no upper bound. */
  maxArea: number;
  includeSocial: boolean;
}

/** A district's rents next to sale prices of the same kind of home; null below the sample minimum. */
export interface RentalDistrict {
  district: string;
  rentals: number;
  medianRentPerPing: number | null;
  medianMonthlyRent: number | null;
  sales: number;
  medianPricePerPing: number | null;
  grossYield: number | null;
}

export interface RentalReport {
  cityCode: string;
  /** First and last month, YYYY-MM; empty without imported leases. */
  from: string;
  through: string;
  minimumSample: number;
  includeSocialHousing: boolean;
  city: RentalDistrict;
  districts: RentalDistrict[];
}

/** Median rent per ping of the leases of a month and the two before it. */
export interface RentPoint {
  month: string;
  rentals: number;
  medianRentPerPing: number | null;
  provisional: boolean;
}

export interface RentTrend {
  cityCode: string;
  district: string;
  through: string;
  minimumSample: number;
  city: RentPoint[];
  districtPoints: RentPoint[];
}

export interface StationRentRing {
  radius: StationRadius;
  rentals: number;
  medianRentPerPing: number | null;
  medianMonthlyRent: number | null;
  sales: number;
  medianPricePerPing: number | null;
  grossYield: number | null;
  relativeToDistrict: number | null;
  trend: RentPoint[];
}

export interface StationRent {
  stationId: string;
  cityCode: string;
  district: string;
  from: string;
  through: string;
  minimumSample: number;
  districtMedianRentPerPing: number | null;
  rings: StationRentRing[];
}

export interface EstimateQuery {
  /** Door number such as 台北市大安區仁愛路四段27巷3號. */
  address: string;
  lat: number;
  lng: number;
  type: "住宅大樓" | "華廈" | "公寓" | "透天厝";
  /** Building area without parking, in ping. */
  area: number;
  age: number | null;
  /** 0 when unknown. */
  floor: number;
  totalFloors: number;
  parking: boolean;
}

export interface Comparable {
  id: string;
  address: string;
  lat: number;
  lng: number;
  transactedAt: string;
  areaPing: number;
  age: number | null;
  floorLabel: string;
  pricePerPing: number;
  adjustedPricePerPing: number;
  distanceM: number;
  sameBuilding: boolean;
  /** Share of all weights. */
  weight: number;
}

export type EstimateConfidence = "high" | "medium" | "low";

export interface Estimate {
  cityCode: string;
  district: string;
  /** Month prices are adjusted to, YYYY-MM. */
  asOf: string;
  sufficient: boolean;
  minimumSample: number;
  pricePerPing: number | null;
  lowPricePerPing: number | null;
  highPricePerPing: number | null;
  buildingPrice: number | null;
  parkingPrice: number | null;
  totalPrice: number | null;
  lowPrice: number | null;
  highPrice: number | null;
  confidence: EstimateConfidence | "";
  spread: number;
  backtestMedianError: number;
  rangeLow: number;
  rangeHigh: number;
  /** The heaviest comparable sales, at most 100 of comparableSales. */
  comparables: Comparable[];
  comparableSales: number;
  sameBuildingSales: number;
  maxDistanceM: number;
  /** Null without a price estimate. */
  rent: RentEstimate | null;
}

export interface ComparableLease {
  id: string;
  address: string;
  lat: number;
  lng: number;
  leasedAt: string;
  areaPing: number;
  age: number | null;
  monthlyRent: number;
  rentPerPing: number;
  adjustedRentPerPing: number;
  distanceM: number;
  sameBuilding: boolean;
  weight: number;
}

export interface RentEstimate {
  asOf: string;
  /** False for houses (透天厝), which have too few leases. */
  supported: boolean;
  sufficient: boolean;
  minimumSample: number;
  rentPerPing: number | null;
  monthlyRent: number | null;
  lowMonthlyRent: number | null;
  highMonthlyRent: number | null;
  confidence: EstimateConfidence | "";
  spread: number;
  backtestMedianError: number;
  rangeLow: number;
  rangeHigh: number;
  /** Twelve months of rent over the estimated price without parking. */
  grossYield: number | null;
  districtGrossYield: number | null;
  leases: ComparableLease[];
  leaseCount: number;
  sameBuildingLeases: number;
}

export interface BuildingProfile {
  address: string;
  sales: number;
  category: string;
  completedYear: number | null;
  totalFloors: number | null;
  lastSale: string;
}

export type FacilityGroup = "daily" | "education" | "medical" | "leisure" | "nuisance";

export interface Facility {
  id: string;
  source: "osm" | "nhi" | "moe";
  category: string;
  name: string;
  brand?: string;
  cityCode: string;
  district: string;
  address?: string;
  lat: number;
  lng: number;
  areaM2?: number;
}

export interface FacilityList {
  facilities: Facility[];
  /** More facilities were in the bounds than returned. */
  truncated: boolean;
}

export interface NearbyCategory {
  category: string;
  group: FacilityGroup;
  /** Facilities within the radius. */
  count: number;
  nearestName?: string;
  /** Distance to the nearest within the search distance, or null. */
  nearestM: number | null;
}

export interface NearbyFacilities {
  radiusM: number;
  searchM: number;
  categories: NearbyCategory[];
}

/** An H3 cell of the density layer: kinds of positive facilities within about 500 m. */
export interface FacilityCell {
  hex: string;
  score: number;
  nuisances: number;
  groups: Partial<Record<FacilityGroup, number>>;
}

export interface DistrictFacilities {
  district: string;
  /** Built-up area from door-number cells, km². */
  areaKm2: number;
  counts: Record<string, number>;
}

export interface CityFacilities {
  cityCode: string;
  city: DistrictFacilities;
  districts: DistrictFacilities[];
}

export interface PresaleProject {
  /** city|district|name */
  key: string;
  name: string;
  cityCode: string;
  district: string;
  address: string;
  lat: number | null;
  lng: number | null;
  /** address, nearby, road or district: how precisely the project is located. */
  geocodePrecision: string;
  category: string;
  totalFloors: number | null;
  /** Valid sales: house and land, not cancelled, not special. */
  sales: number;
  cancelled: number;
  recentSales: number;
  firstContract: string;
  lastContract: string;
  medianPricePerPing: number | null;
  medianTotalPrice: number | null;
  medianAreaPing: number | null;
  distanceM?: number;
}

export interface PresaleProjects {
  cityCode: string;
  from: string;
  projects: PresaleProject[];
}

export interface PresaleUnit {
  id: string;
  unitLabel?: string;
  floorLabel: string;
  totalFloors?: number;
  transactionType: string;
  totalPrice: number;
  parkingPrice: number;
  buildingAreaPing: number;
  pricePerPing: number;
  rooms: number;
  transactedAt: string;
  cancellation?: string;
  cancelledOn?: string;
  specialReasons?: string[];
}

export interface ResaleComparison {
  radiusM: number;
  from: string;
  through: string;
  projectPricePerPing: number | null;
  projectSales: number;
  projectFrom: string;
  projectThrough: string;
  resales: number;
  resalePricePerPing: number | null;
  newResales: number;
  newPricePerPing: number | null;
  premium: number | null;
  newPremium: number | null;
}

export interface PresaleProjectDetail {
  project: PresaleProject;
  lowPricePerPing: number | null;
  highPricePerPing: number | null;
  mainRooms: number | null;
  months: { month: string; sales: number; medianPricePerPing: number | null }[];
  floors: { floor: number; sales: number; medianPricePerPing: number }[];
  units: PresaleUnit[];
  resale: ResaleComparison | null;
}

export interface InvestmentQuery {
  city: string;
  type: BuildingType;
  minArea: number;
  maxArea: number;
  minAge: number;
  maxAge: number;
}

export interface InvestmentArea {
  district: string;
  currentCount: number;
  previousCount: number;
  medianPricePerPing: number | null;
  volumeChange: number | null;
  priceChange: number | null;
  relativeToCityMedian: number | null;
  sufficientCurrentSample: boolean;
  sufficientComparisonSample: boolean;
}

export interface InvestmentReport {
  cityCode: string;
  through: string;
  currentFrom: string;
  previousFrom: string;
  minimumSample: number;
  cityCurrentCount: number;
  cityPreviousCount: number;
  cityMedianPricePerPing: number | null;
  cityVolumeChange: number | null;
  cityPriceChange: number | null;
  areas: InvestmentArea[];
}

export interface InvestmentWorkbenchArea extends InvestmentArea {
  rentalCount: number;
  medianRentPerPing: number | null;
  /** Ratio of distinct district median rent and sale unit prices, not property cash flow. */
  grossYield: number | null;
}

export interface InvestmentWorkbenchReport {
  cityCode: string;
  through: string;
  currentFrom: string;
  previousFrom: string;
  minimumSales: number;
  minimumRentals: number;
  areas: InvestmentWorkbenchArea[];
}

export interface InvestmentLiquidityMonth {
  month: string;
  count: number;
  /** Imported registrations cover this month's 90-day registration window. */
  coverageComplete: boolean;
}

export interface InvestmentLiquidityArea {
  district: string;
  currentCount: number;
  previousCount: number;
  activeMonths: number | null;
  longestZeroRun: number | null;
  volumeChange: number | null;
  sufficientCurrentSample: boolean;
  sufficientComparisonSample: boolean;
  monthly: InvestmentLiquidityMonth[];
}

export interface InvestmentLiquidityReport {
  cityCode: string;
  through: string;
  currentFrom: string;
  previousFrom: string;
  minimumSample: number;
  currentCoverageComplete: boolean;
  previousCoverageComplete: boolean;
  incompleteMonths: string[];
  areas: InvestmentLiquidityArea[];
}

export interface Place {
  kind: "district" | "road" | "address" | "station";
  label: string;
  lat: number;
  lng: number;
  /** Set for stations, e.g. TRTC-BL07. */
  stationId?: string;
}

export interface TransactionQuery {
  priceRange: [number, number];
  areaRange: [number, number];
  types: BuildingType[];
  bbox?: BBox;
  /** Limits results to one month (YYYY-MM); otherwise the API's 12-month window applies. */
  month?: string;
  includeSpecial?: boolean;
  /** Leave out sales of buildings under two years old, e.g. a development delivering at once. */
  excludeNewBuildings?: boolean;
}

export const FEED_SORTS = ["date_desc", "date_asc", "price_per_ping_desc", "price_per_ping_asc", "total_price_desc"] as const;
export type FeedSort = (typeof FEED_SORTS)[number];

export interface FeedQuery extends TransactionQuery {
  /** First and last month (YYYY-MM) of sales to include; the API defaults to 12 months. */
  from?: string;
  to?: string;
  city?: string;
  district?: string;
  latestOnly?: boolean;
  sort: FeedSort;
}

export interface AnalysisQuery extends TransactionQuery {
  from: string;
  to: string;
}

export interface Release {
  label: string;
  registeredFrom: string;
  registeredTo: string;
}

export interface FeedPage {
  data: TransactionData[];
  meta: {
    total?: number;
    newCount?: number;
    nextCursor?: string;
    latestRelease: Release | null;
  };
}

export interface Region {
  code: string;
  name: string;
  districts: string[];
}

/** Sales inside one hexagon, for the map's hover card. */
export interface CellDetail {
  hex: string;
  count: number;
  avgPricePerPing: number;
  avgTotalPrice: number;
  avgArea: number;
  cityCode: string;
  district: string;
  road: string;
  /** Area-weighted price change over 12 fully registered months; null with too few sales. */
  priceYoY: number | null;
  yoyThrough: string;
  ageMix: { key: "0-5" | "5-15" | "15-30" | "30+" | "unknown"; count: number }[];
}

/** GeoJSON FeatureCollection of district areas approximated from door numbers. */
export interface DistrictBoundaries {
  type: "FeatureCollection";
  features: {
    type: "Feature";
    properties: { district: string; cells: number };
    geometry: { type: "MultiPolygon"; coordinates: number[][][][] };
  }[];
}

export type TransitSystem = "metro" | "lrt" | "tra" | "thsr";

/** GeoJSON FeatureCollection of rail routes (MRT, light rail, 台鐵, 高鐵). */
export interface TransitLines {
  type: "FeatureCollection";
  features: {
    type: "Feature";
    properties: { id: string; operator: string; system: TransitSystem; name: string; nameEn: string; color: string; openedOn: string | null };
    geometry: { type: "MultiLineString"; coordinates: number[][][] };
  }[];
}

export interface StationLine {
  lineId: string;
  name: string;
  color: string;
  platformId: string;
  platformName: string;
  sequence: number;
  /** YYYY-MM-DD for platforms opened since 2015; null when older. */
  openedOn: string | null;
}

/** A station merges transfer platforms, e.g. 板橋 for 台鐵, 高鐵, 板南線 and 環狀線. */
export interface TransitStation {
  id: string;
  name: string;
  nameEn: string;
  cityCode: string;
  district: string;
  lat: number;
  lng: number;
  systems: TransitSystem[];
  openedOn: string | null;
  lines: StationLine[];
}

export const STATION_RADII = [300, 500, 800] as const;
export type StationRadius = typeof STATION_RADII[number];

/** Sales within a radius of a station's nearest exit. */
export interface StationRing {
  radius: StationRadius;
  count: number;
  avgPricePerPing: number;
  medianPricePerPing: number;
  priceYoY: number | null;
  /** Monthly trend over all years. */
  trends: TrendPoint[];
}

export interface StationTrend {
  station: TransitStation;
  yoyThrough: string;
  rings: StationRing[];
}

export interface StationYoYs {
  radius: StationRadius;
  yoyThrough: string;
  stations: { stationId: string; count: number; priceYoY: number | null }[];
}

/** A city's located sales [fromM, toM) metres from the nearest station; toM null is beyond. */
export interface DistanceBucket {
  cityCode: string;
  fromM: number;
  toM: number | null;
  count: number;
  avgPricePerPing: number;
  medianPricePerPing: number;
}

/** A station's figures within a radius for a date window. */
export interface StationRank {
  stationId: string;
  count: number;
  avgPricePerPing: number;
  medianPricePerPing: number;
  /** Over the last 12 complete months; null with fewer than 5 sales in either year. */
  priceYoY: number | null;
  /** Monthly trend over the window. */
  trend: TrendPoint[];
}

export interface StationRanking {
  radius: StationRadius;
  yoyThrough: string;
  stations: StationRank[];
}

/** Sales in the 12 months before an opening and the 12 months from its month. */
export interface OpeningWindows {
  beforeCount: number;
  afterCount: number;
  beforePrice: number | null;
  afterPrice: number | null;
  change: number | null;
}

export interface OpeningPoint {
  month: string;
  /** Months from the opening month. */
  offset: number;
  stationPrice: number | null;
  stationCount: number;
  baselinePrice: number | null;
  baselineCount: number;
}

/** A line opening at a station against the district's sales beyond 800 m of it. */
export interface OpeningEffect {
  stationId: string;
  stationName: string;
  cityCode: string;
  district: string;
  lineId: string;
  lineName: string;
  lineColor: string;
  openedOn: string;
  station: OpeningWindows;
  baseline: OpeningWindows;
  /** Station change minus district change; null unless status is ok. */
  relative: number | null;
  status: "ok" | "too_recent" | "insufficient";
  series: OpeningPoint[];
}

export interface OpeningEffects {
  radius: StationRadius;
  events: OpeningEffect[];
}

export type ImportDataset = "sales" | "sales_release" | "rentals" | "rentals_release" | "presales" | "presales_release" | "osm_facilities" | "nhi_facilities" | "moe_schools" | "door_numbers" | "district_boundaries" | "transit";
export type ImportRunState = "succeeded" | "failed" | "running" | "interrupted";
export type ImportErrorKind = "rate_limited" | "unauthorized" | "unpublished" | "missing_input" | "download" | "parse" | "database" | "other";

/** One importer attempt, without its error message. */
export interface ImportRun {
  id: number;
  source: string;
  dataset: ImportDataset;
  /** Season (115S2), 10-day release (1150901-1150910) or month (2026-09). */
  season: string;
  /** Empty for datasets that cover every city, such as rail stations. */
  cityCode: string;
  status: ImportRunState;
  rowsRead: number;
  rowsSkipped: number;
  /** Rows written, including corrections of existing sales; not new sales. */
  rowsUpserted: number;
  rowsGeocodeFailed: number;
  startedAt: string;
  finishedAt: string | null;
  errorKind?: ImportErrorKind;
  /** The full error message; only the admin console receives it. */
  error?: string;
}

export interface CityDataCount {
  cityCode: string;
  transactions: number;
  located: number;
  /** Sales dated after today, left out of latestSaleDate. */
  futureDated: number;
  latestSaleDate: string | null;
  /** Leases (租賃); latestLeaseDate ignores lease dates in the future. */
  rentals: number;
  rentalsLocated: number;
  latestLeaseDate: string | null;
}

export interface SeasonCoverage {
  cityCode: string;
  firstImported: string;
  latestImported: string;
  /** Seasons missing between the first and latest imported ones. */
  missing: string[];
}

export interface DataStatus {
  generatedAt: string;
  recentRuns: ImportRun[];
  latestRuns: ImportRun[];
  lastSuccessfulRuns: ImportRun[];
  cities: CityDataCount[];
  coverage: SeasonCoverage[];
  quality: {
    metrics: QualityMetric[];
    alerts: QualityAlert[];
  };
}

export interface QualityMetric {
  cityCode: string;
  dataset: "sales" | "sales_release";
  season: string;
  rowsRead: number;
  rowsSkipped: number;
  /** Upserts include corrections, not just new sales. */
  rowsUpserted: number;
  rowsGeocodeFailed: number;
  preciseCount: number;
  locatedCount: number;
  preciseRate: number | null;
  locatedRate: number | null;
  previousRowsRead: number | null;
  volumeChange: number | null;
  finishedAt: string;
}

export interface QualityAlert {
  code: "low_location_rate" | "low_precise_rate" | "volume_drop" | "stale_release" | "import_failed";
  severity: "warning" | "error";
  cityCode: string;
  dataset: "sales" | "sales_release";
  season: string;
  runId: number;
}

export interface TransactionAPI {
  list(query: TransactionQuery, signal?: AbortSignal): Promise<TransactionData[]>;
  aggregateH3(query: TransactionQuery, resolution: number, signal?: AbortSignal): Promise<H3Aggregate[]>;
  trends(query: TransactionQuery, signal?: AbortSignal): Promise<TrendPoint[]>;
  summary(query: TransactionQuery, signal?: AbortSignal): Promise<SummaryWithYoY>;
  analysis(query: AnalysisQuery, signal?: AbortSignal): Promise<MarketAnalysis>;
  compareDistricts(query: DistrictQuery, signal?: AbortSignal): Promise<DistrictComparison>;
  investmentAreas(query: InvestmentQuery, signal?: AbortSignal): Promise<InvestmentReport>;
  investmentWorkbench(query: InvestmentQuery, signal?: AbortSignal): Promise<InvestmentWorkbenchReport>;
  investmentLiquidity(query: InvestmentQuery, signal?: AbortSignal): Promise<InvestmentLiquidityReport>;
  /** Rents and gross yields by district over the last 12 complete months. */
  rentalDistricts(query: RentalQuery, signal?: AbortSignal): Promise<RentalReport>;
  /** The city's and, when given, a district's rent trend over five years. */
  rentalTrend(query: RentalQuery, district: string, signal?: AbortSignal): Promise<RentTrend>;
  facilities(bbox: BBox, categories: string[], signal?: AbortSignal): Promise<FacilityList>;
  nearbyFacilities(lat: number, lng: number, radius: number, signal?: AbortSignal): Promise<NearbyFacilities>;
  districtFacilities(city: string, signal?: AbortSignal): Promise<CityFacilities>;
  facilityDensity(bbox: BBox, resolution: 8 | 9, categories: string[], signal?: AbortSignal): Promise<FacilityCell[]>;
  /** Presale projects of a city with a contract in the last months months, or near a point. */
  presaleProjects(query: { city: string; district?: string; months: number; near?: { lat: number; lng: number; radius: number }; bbox?: BBox }, signal?: AbortSignal): Promise<PresaleProjects>;
  presaleProject(key: string, signal?: AbortSignal): Promise<PresaleProjectDetail>;
  /** Values a home from comparable sales nearby. */
  estimate(query: EstimateQuery, signal?: AbortSignal): Promise<Estimate>;
  /** Recorded sales of the building at an address, to fill in the estimate form. */
  estimateBuilding(address: string, lat: number, lng: number, signal?: AbortSignal): Promise<BuildingProfile>;
  /** Rents within each ring of a station, excluding social housing. */
  stationRent(stationId: string, signal?: AbortSignal): Promise<StationRent>;
  searchPlaces(text: string, signal?: AbortSignal): Promise<Place[]>;
  feed(query: FeedQuery, cursor?: string, signal?: AbortSignal): Promise<FeedPage>;
  regions(signal?: AbortSignal): Promise<Region[]>;
  /** URL of a CSV of the query's sales (up to 50,000), for a download link. */
  exportUrl(query: FeedQuery): string;
  districtBoundaries(city: string, signal?: AbortSignal): Promise<DistrictBoundaries>;
  cellDetail(query: TransactionQuery, hex: string, signal?: AbortSignal): Promise<CellDetail>;
  transitLines(signal?: AbortSignal): Promise<TransitLines>;
  transitStations(signal?: AbortSignal): Promise<TransitStation[]>;
  /** Every station's YoY within radius; uses the query's filters but not its area or month. */
  stationYoYs(query: TransactionQuery, radius: StationRadius, signal?: AbortSignal): Promise<StationYoYs>;
  /** Figures for the query's month (or the last 12 months) and trends over all years. */
  stationTrend(query: TransactionQuery, stationId: string, signal?: AbortSignal): Promise<StationTrend>;
  /** Every station's figures within radius from the given month (default: last 12 months). */
  stationRanking(query: TransactionQuery, radius: StationRadius, from?: string, signal?: AbortSignal): Promise<StationRanking>;
  /** Price per ping by distance to the nearest station, by city, from the given month. */
  distanceCurve(query: TransactionQuery, from?: string, signal?: AbortSignal): Promise<DistanceBucket[]>;
  /** Price changes around each line opening; ignores the query's dates. */
  openingEffects(query: TransactionQuery, radius: StationRadius, signal?: AbortSignal): Promise<OpeningEffects>;
}

export function buildingCategory(buildingType: string): BuildingType {
  const category = buildingType.split("(")[0];
  return (BUILDING_TYPES as readonly string[]).includes(category) ? category as BuildingType : "其他";
}
