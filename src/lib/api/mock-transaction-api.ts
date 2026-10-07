import { cellsToMultiPolygon, cellToLatLng, getResolution, latLngToCell } from "h3-js";
import { generateMockTransactions } from "../mock-data";
import { BUILDING_TYPES, buildingCategory } from "./types";
import type { InvestmentLiquidityReport } from "./types";
import type { DistanceBucket, OpeningEffects, StationRanking, CellDetail, DistrictBoundaries, StationRadius, StationTrend, StationYoYs, TransitLines, TransitStation, AnalysisQuery, BreakdownAnalysis, CategoryAnalysis, DistrictComparison, DistrictQuery, FeedPage, FeedQuery, H3Aggregate, InvestmentQuery, BuildingProfile, CityFacilities, Estimate, FacilityCell, FacilityList, NearbyFacilities, PresaleProjectDetail, PresaleProjects, RentalQuery, RentalReport, RentTrend, StationRent, InvestmentReport, MarketAnalysis, MonthlyAnalysis, Place, PriceBucket, Region, SummaryWithYoY, TransactionAPI, TransactionData, TransactionQuery, TrendPoint } from "./types";

function mulberry32(seed: number): () => number {
  return () => {
    seed |= 0;
    seed = seed + 0x6d2b79f5 | 0;
    let value = Math.imul(seed ^ seed >>> 15, 1 | seed);
    value = value + Math.imul(value ^ value >>> 7, 61 | value) ^ value;
    return ((value ^ value >>> 14) >>> 0) / 4294967296;
  };
}

interface Accumulator {
  count: number;
  totalPrice: number;
  buildingPrice: number;
  area: number;
}

interface AnalysisAccumulator extends Accumulator {
  unitPrices: number[];
  totalPrices: number[];
}

function analysisAccumulator(): AnalysisAccumulator {
  return { count: 0, totalPrice: 0, buildingPrice: 0, area: 0, unitPrices: [], totalPrices: [] };
}

function addAnalysis(group: AnalysisAccumulator, transaction: TransactionData) {
  group.count++;
  group.totalPrice += transaction.totalPrice;
  group.buildingPrice += transaction.buildingPrice;
  group.area += transaction.buildingAreaPing;
  group.unitPrices.push(transaction.pricePerPing);
  group.totalPrices.push(transaction.totalPrice);
}

function median(values: number[]) {
  if (values.length === 0) return 0;
  const sorted = [...values].sort((a, b) => a - b);
  const middle = Math.floor(sorted.length / 2);
  return sorted.length % 2 ? sorted[middle] : (sorted[middle - 1] + sorted[middle]) / 2;
}

function accumulate(groups: Map<string, Accumulator>, key: string, transaction: TransactionData) {
  const group = groups.get(key) ?? { count: 0, totalPrice: 0, buildingPrice: 0, area: 0 };
  group.count++;
  group.totalPrice += transaction.totalPrice;
  group.buildingPrice += transaction.buildingPrice;
  group.area += transaction.buildingAreaPing;
  groups.set(key, group);
}

export class MockTransactionAPI implements TransactionAPI {
  private readonly transactions = generateMockTransactions(3000, mulberry32(591));

  async list(query: TransactionQuery): Promise<TransactionData[]> {
    return this.filtered(query).slice(0, 2000);
  }

  async aggregateH3(query: TransactionQuery, resolution: number): Promise<H3Aggregate[]> {
    const groups = new Map<string, Accumulator>();
    for (const transaction of this.filtered(query)) {
      accumulate(groups, latLngToCell(transaction.lat, transaction.lng, resolution), transaction);
    }

    return Array.from(groups, ([hex, group]) => {
      const [lat, lng] = cellToLatLng(hex);
      return {
        hex,
        lat,
        lng,
        count: group.count,
        price: group.totalPrice / group.count,
        avgArea: group.area / group.count,
        avgPricePerPing: group.buildingPrice / group.area,
      };
    });
  }

  async trends(query: TransactionQuery): Promise<TrendPoint[]> {
    const groups = new Map<string, Accumulator>();
    for (const transaction of this.filtered(query)) {
      accumulate(groups, transaction.transactedAt.slice(0, 7), transaction);
    }

    const months = Array.from(groups.keys()).sort();
    const provisionalFrom = months[months.length - 2] ?? "";
    return Array.from(groups, ([month, group]) => ({
      month,
      count: group.count,
      avgPricePerPing: group.buildingPrice / group.area,
      provisional: month >= provisionalFrom,
    })).sort((a, b) => a.month.localeCompare(b.month));
  }

  // Mock data covers only the last 12 months, so there is no previous year to compare.
  async summary(query: TransactionQuery): Promise<SummaryWithYoY> {
    const yoy = { basis: query.month ? "3m" : "12m", through: query.month ?? "", count: null, avgPricePerPing: null, medianTotalPrice: null };
    const transactions = this.filtered(query);
    if (transactions.length === 0) return { count: 0, avgPricePerPing: 0, medianTotalPrice: 0, yoy };
    const prices = transactions.map((transaction) => transaction.totalPrice).sort((a, b) => a - b);
    const middle = Math.floor(prices.length / 2);
    const area = transactions.reduce((sum, transaction) => sum + transaction.buildingAreaPing, 0);
    const buildingPrice = transactions.reduce((sum, transaction) => sum + transaction.buildingPrice, 0);
    return {
      count: transactions.length,
      avgPricePerPing: buildingPrice / area,
      medianTotalPrice: prices.length % 2 ? prices[middle] : (prices[middle - 1] + prices[middle]) / 2,
      yoy,
    };
  }

  async analysis(query: AnalysisQuery): Promise<MarketAnalysis> {
    const transactions = this.filtered(query).filter((transaction) => {
      const month = transaction.transactedAt.slice(0, 7);
      return month >= query.from && month <= query.to;
    });
    const months = new Map<string, AnalysisAccumulator>();
    const categoryGroups = new Map<string, AnalysisAccumulator>();
    const ageGroups = new Map<string, AnalysisAccumulator>();
    const areaGroups = new Map<string, AnalysisAccumulator>();
    const distribution = new Map<number, number>();
    for (const transaction of transactions) {
      const month = transaction.transactedAt.slice(0, 7);
      const monthGroup = months.get(month) ?? analysisAccumulator();
      addAnalysis(monthGroup, transaction);
      months.set(month, monthGroup);

      const category = buildingCategory(transaction.buildingType);
      const categoryGroup = categoryGroups.get(category) ?? analysisAccumulator();
      addAnalysis(categoryGroup, transaction);
      categoryGroups.set(category, categoryGroup);

      const age = transaction.completedAt
        ? Math.floor((new Date(transaction.transactedAt).getTime() - new Date(transaction.completedAt).getTime()) / (365.25 * 86_400_000))
        : -1;
      const ageKey = age < 0 ? "unknown" : age < 5 ? "under5" : age < 10 ? "5_9" : age < 20 ? "10_19" : age < 30 ? "20_29" : "30plus";
      const ageGroup = ageGroups.get(ageKey) ?? analysisAccumulator();
      addAnalysis(ageGroup, transaction);
      ageGroups.set(ageKey, ageGroup);

      const area = transaction.buildingAreaPing;
      const areaKey = area < 15 ? "under15" : area < 30 ? "15_29" : area < 45 ? "30_44" : "45plus";
      const areaGroup = areaGroups.get(areaKey) ?? analysisAccumulator();
      addAnalysis(areaGroup, transaction);
      areaGroups.set(areaKey, areaGroup);

      const lower = Math.floor(transaction.pricePerPing / 100_000) * 100_000;
      distribution.set(lower, (distribution.get(lower) ?? 0) + 1);
    }
    const summarizeMonth = ([month, group]: [string, AnalysisAccumulator]): MonthlyAnalysis => ({
      month,
      count: group.count,
      avgPricePerPing: group.buildingPrice / group.area,
      medianPricePerPing: median(group.unitPrices),
      medianTotalPrice: median(group.totalPrices),
      avgArea: group.area / group.count,
      provisional: month >= new Date(Date.now() - 90 * 86_400_000).toISOString().slice(0, 7),
    });
    const categories: CategoryAnalysis[] = Array.from(categoryGroups, ([category, group]) => ({
      category: category as CategoryAnalysis["category"],
      count: group.count,
      avgPricePerPing: group.buildingPrice / group.area,
      medianPricePerPing: median(group.unitPrices),
      medianTotalPrice: median(group.totalPrices),
    }));
    const buckets: PriceBucket[] = Array.from(distribution, ([minPricePerPing, count]) => ({
      minPricePerPing, maxPricePerPing: minPricePerPing + 100_000, count,
    }));
    const breakdown = (groups: Map<string, AnalysisAccumulator>, order: string[]): BreakdownAnalysis[] =>
      order.filter((key) => groups.has(key)).map((key) => {
        const group = groups.get(key)!;
        return { key, count: group.count, avgPricePerPing: group.buildingPrice / group.area,
          medianPricePerPing: median(group.unitPrices), medianTotalPrice: median(group.totalPrices) };
      });
    return {
      monthly: Array.from(months).map(summarizeMonth).sort((a, b) => a.month.localeCompare(b.month)),
      categories: categories.sort((a, b) => BUILDING_TYPES.indexOf(a.category) - BUILDING_TYPES.indexOf(b.category)),
      ageGroups: breakdown(ageGroups, ["under5", "5_9", "10_19", "20_29", "30plus", "unknown"]),
      areaGroups: breakdown(areaGroups, ["under15", "15_29", "30_44", "45plus"]),
      distribution: buckets.sort((a, b) => a.minPricePerPing - b.minPricePerPing),
    };
  }

  async compareDistricts(query: DistrictQuery): Promise<DistrictComparison> {
    const rows = this.filtered(query).filter((transaction) => transaction.cityCode === query.city &&
      transaction.transactedAt.slice(0, 7) >= query.from && transaction.transactedAt.slice(0, 7) <= query.to);
    const ranks = new Map<string, AnalysisAccumulator>();
    const trends = new Map<string, AnalysisAccumulator>();
    for (const transaction of rows) {
      const rank = ranks.get(transaction.district) ?? analysisAccumulator();
      addAnalysis(rank, transaction);
      ranks.set(transaction.district, rank);
      if (!query.districts.includes(transaction.district)) continue;
      const key = `${transaction.district}|${transaction.transactedAt.slice(0, 7)}`;
      const trend = trends.get(key) ?? analysisAccumulator();
      addAnalysis(trend, transaction);
      trends.set(key, trend);
    }
    const provisionalFrom = new Date(Date.now() - 90 * 86_400_000).toISOString().slice(0, 7);
    return {
      cityCode: query.city,
      yoyThrough: "",
      ranking: Array.from(ranks, ([district, group]) => ({
        district, count: group.count, avgPricePerPing: group.buildingPrice / group.area,
        medianPricePerPing: median(group.unitPrices), medianTotalPrice: median(group.totalPrices), priceYoY: null,
      })).sort((a, b) => b.medianPricePerPing - a.medianPricePerPing || a.district.localeCompare(b.district)),
      trends: Array.from(trends, ([key, group]) => {
        const [district, month] = key.split("|");
        return { district, month, count: group.count, avgPricePerPing: group.buildingPrice / group.area, provisional: month >= provisionalFrom };
      }).sort((a, b) => a.month.localeCompare(b.month) || a.district.localeCompare(b.district)),
    };
  }

  // The mock has no leases: rent pages show their empty state.
  async rentalDistricts(query: RentalQuery): Promise<RentalReport> {
    const empty = { district: "", rentals: 0, medianRentPerPing: null, medianMonthlyRent: null, sales: 0, medianPricePerPing: null, grossYield: null };
    return { cityCode: query.city, from: "", through: "", minimumSample: 20, includeSocialHousing: query.includeSocial, city: empty, districts: [] };
  }

  async rentalTrend(query: RentalQuery, district: string): Promise<RentTrend> {
    return { cityCode: query.city, district, through: "", minimumSample: 10, city: [], districtPoints: [] };
  }

  // The mock has no facilities.
  async facilities(): Promise<FacilityList> {
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

  // The mock has no presales.
  async presaleProjects(query: { city: string; months: number }): Promise<PresaleProjects> {
    return { cityCode: query.city, from: "", projects: [] };
  }

  async presaleProject(key: string): Promise<PresaleProjectDetail> {
    throw new Error(`no presale project ${key}`);
  }

  // The mock has no door numbers to match buildings, so estimates are always insufficient.
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

  async stationRent(stationId: string): Promise<StationRent> {
    return { stationId, cityCode: "a", district: "", from: "", through: "", minimumSample: 20, districtMedianRentPerPing: null, rings: [] };
  }

  async investmentAreas(query: InvestmentQuery): Promise<InvestmentReport> {
    const month = new Date();
    month.setUTCDate(1);
    month.setUTCMonth(month.getUTCMonth() - 4);
    const label = (date: Date) => date.toISOString().slice(0, 7);
    const shift = (date: Date, months: number) => new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth() + months, 1));
    const through = label(month), currentFrom = label(shift(month, -11)), previousFrom = label(shift(month, -23));
    const groups = new Map<string, { current: number[]; previous: number[] }>();
    groups.set("", { current: [], previous: [] });
    for (const sale of this.transactions) {
      const soldMonth = sale.transactedAt.slice(0, 7);
      if (sale.cityCode !== query.city || buildingCategory(sale.buildingType) !== query.type ||
        sale.mainUse !== "住家用" || !sale.transactionType.startsWith("房地") ||
        sale.specialReasons?.length || sale.buildingAreaPing < query.minArea || sale.buildingAreaPing >= query.maxArea ||
        !sale.completedAt || soldMonth < previousFrom || soldMonth > through) continue;
      const age = Math.floor((Date.parse(sale.transactedAt) - Date.parse(sale.completedAt)) / 86_400_000 / 365.25);
      if (age < query.minAge || age >= query.maxAge) continue;
      if (!groups.has(sale.district)) groups.set(sale.district, { current: [], previous: [] });
      for (const key of ["", sale.district]) groups.get(key)![soldMonth >= currentFrom ? "current" : "previous"].push(sale.pricePerPing);
    }
    const city = groups.get("")!;
    const cityMedian = city.current.length >= 30 ? median(city.current) : null;
    const areas = Array.from(groups).filter(([district]) => district).map(([district, group]) => {
      const currentCount = group.current.length, previousCount = group.previous.length;
      const sufficientCurrentSample = currentCount >= 30;
      const sufficientComparisonSample = sufficientCurrentSample && previousCount >= 30;
      const medianPricePerPing = sufficientCurrentSample ? median(group.current) : null;
      return {
        district, currentCount, previousCount, medianPricePerPing,
        volumeChange: sufficientComparisonSample ? currentCount / previousCount - 1 : null,
        priceChange: sufficientComparisonSample ? median(group.current) / median(group.previous) - 1 : null,
        relativeToCityMedian: medianPricePerPing !== null && cityMedian !== null ? medianPricePerPing / cityMedian - 1 : null,
        sufficientCurrentSample, sufficientComparisonSample,
      };
    }).sort((a, b) => b.currentCount - a.currentCount || a.district.localeCompare(b.district));
    const cityComparable = city.current.length >= 30 && city.previous.length >= 30;
    return { cityCode: query.city, through, currentFrom, previousFrom, minimumSample: 30,
      cityCurrentCount: city.current.length, cityPreviousCount: city.previous.length, cityMedianPricePerPing: cityMedian,
      cityVolumeChange: cityComparable ? city.current.length / city.previous.length - 1 : null,
      cityPriceChange: cityComparable ? median(city.current) / median(city.previous) - 1 : null, areas };
  }

  async investmentWorkbench(query: InvestmentQuery) {
    const sales = await this.investmentAreas(query);
    return {cityCode: sales.cityCode, through: "", currentFrom: "", previousFrom: "", minimumSales: 30, minimumRentals: 20, areas: []};
  }

  async investmentLiquidity(query: InvestmentQuery): Promise<InvestmentLiquidityReport> {
    return { cityCode: query.city, through: "", currentFrom: "", previousFrom: "", minimumSample: 30,
      currentCoverageComplete: false, previousCoverageComplete: false, incompleteMonths: [], areas: [] };
  }

  async feed(query: FeedQuery, cursor?: string): Promise<FeedPage> {
    const key = (transaction: TransactionData) =>
      query.sort.startsWith("date") ? transaction.transactedAt : query.sort.startsWith("price_per_ping") ? transaction.pricePerPing : transaction.totalPrice;
    const descending = query.sort.endsWith("desc");
    const rows = this.filtered({ ...query, month: undefined })
      .filter((transaction) => !query.district || transaction.district === query.district)
      .sort((a, b) => (key(a) < key(b) ? -1 : key(a) > key(b) ? 1 : a.id.localeCompare(b.id)) * (descending ? -1 : 1));
    const start = cursor ? rows.findIndex((transaction) => transaction.id === cursor) + 1 : 0;
    const page = rows.slice(start, start + 50);
    return {
      data: page,
      meta: {
        total: cursor ? undefined : rows.length,
        newCount: cursor ? undefined : 0,
        nextCursor: start + 50 < rows.length ? page[page.length - 1].id : undefined,
        latestRelease: null,
      },
    };
  }

  async regions(): Promise<Region[]> {
    return [{ code: "a", name: "台北市", districts: [...new Set(this.transactions.map((transaction) => transaction.district))] }];
  }

  async cellDetail(query: TransactionQuery, hex: string): Promise<CellDetail> {
    const sales = this.filtered(query).filter((sale) => latLngToCell(sale.lat, sale.lng, getResolution(hex)) === hex);
    const area = sales.reduce((sum, sale) => sum + sale.buildingAreaPing, 0);
    const districts = new Map<string, number>();
    for (const sale of sales) districts.set(sale.district, (districts.get(sale.district) ?? 0) + 1);
    const district = [...districts].sort((a, b) => b[1] - a[1])[0]?.[0] ?? "";
    return {
      hex,
      count: sales.length,
      avgPricePerPing: area ? sales.reduce((sum, sale) => sum + sale.buildingPrice, 0) / area : 0,
      avgTotalPrice: sales.length ? sales.reduce((sum, sale) => sum + sale.totalPrice, 0) / sales.length : 0,
      avgArea: sales.length ? area / sales.length : 0,
      cityCode: "a",
      district,
      road: "",
      priceYoY: null,
      yoyThrough: "",
      // Mock sales have no completion dates.
      ageMix: [{ key: "unknown", count: sales.length }],
    };
  }

  // Like the API, derive district areas from where sales are, merging resolution-8 cells.
  async districtBoundaries(): Promise<DistrictBoundaries> {
    const cells = new Map<string, Set<string>>();
    for (const transaction of this.transactions) {
      const set = cells.get(transaction.district) ?? new Set<string>();
      set.add(latLngToCell(transaction.lat, transaction.lng, 8));
      cells.set(transaction.district, set);
    }
    return {
      type: "FeatureCollection",
      features: Array.from(cells, ([district, set]) => ({
        type: "Feature" as const,
        properties: { district, cells: set.size },
        geometry: { type: "MultiPolygon" as const, coordinates: cellsToMultiPolygon([...set], true).map((polygon) => [polygon[0]]) },
      })),
    };
  }

  exportUrl(): string {
    return "";
  }

  // Mock data has no rail network; stations come from the TDX import.
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
    throw new Error("Mock data has no stations");
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


  async searchPlaces(text: string): Promise<Place[]> {
    const groups = new Map<string, TransactionData[]>();
    for (const transaction of this.transactions) {
      if (!transaction.district.startsWith(text) && !transaction.address.includes(text)) continue;
      groups.set(transaction.district, [...(groups.get(transaction.district) ?? []), transaction]);
    }
    return Array.from(groups, ([district, matches]) => ({
      kind: "district" as const,
      label: `台北市${district}`,
      lat: matches.reduce((sum, transaction) => sum + transaction.lat, 0) / matches.length,
      lng: matches.reduce((sum, transaction) => sum + transaction.lng, 0) / matches.length,
    })).slice(0, 8);
  }

  private filtered(query: TransactionQuery): TransactionData[] {
    return this.transactions.filter((transaction) => {
      const insideBBox = !query.bbox || (
        transaction.lng >= query.bbox[0] && transaction.lat >= query.bbox[1] &&
        transaction.lng <= query.bbox[2] && transaction.lat <= query.bbox[3]
      );
      return insideBBox &&
        transaction.totalPrice >= query.priceRange[0] && transaction.totalPrice <= query.priceRange[1] &&
        transaction.buildingAreaPing >= query.areaRange[0] && transaction.buildingAreaPing <= query.areaRange[1] &&
        query.types.includes(buildingCategory(transaction.buildingType)) &&
        (!query.month || transaction.transactedAt.startsWith(query.month)) &&
        (query.includeSpecial || !transaction.specialReasons?.length);
    });
  }
}
