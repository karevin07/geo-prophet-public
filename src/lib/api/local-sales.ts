import { cellToLatLng, getResolution, isValidCell, latLngToCell } from "h3-js";
import { BUILDING_TYPES, buildingCategory } from "./types";
import type { AnalysisQuery, BreakdownAnalysis, BuildingType, CellDetail, DistrictComparison, DistrictQuery, FeedPage, FeedQuery, H3Aggregate, MarketAnalysis, Place, Region, Release, SummaryWithYoY, TransactionData, TransactionQuery, TrendPoint } from "./types";

// Answers the API's sales queries in the browser, for the online demo (from a snapshot of
// real sales) and the mock mode (from generated ones). Each method follows the Go handler,
// usecase and SQL it replaces, so the demo shows what the API would.

/** A sale with the road parsed from its address, as the API derives it for cell cards. */
export interface LocalSale extends TransactionData {
  road: string;
}

export interface LocalSalesOptions {
  region: Region;
  /** The last fully registered month (YYYY-MM); null when unknown. */
  completeMonth: string | null;
  latestRelease: Release | null;
  now?: () => Date;
}

interface Filter {
  minPrice: number;
  maxPrice: number;
  minArea: number;
  maxArea: number;
  includeSpecial: boolean;
  preciseLocation: boolean;
  excludeNewBuildings: boolean;
  categories: string[];
  bbox?: [number, number, number, number];
  /** First and last day (YYYY-MM-DD), inclusive. */
  from: string;
  to: string;
}

interface Row {
  sale: LocalSale;
  day: string;
  month: string;
  category: BuildingType;
  /** Whole years from completion to sale, null when unknown, as the building_age column. */
  age: number | null;
}

/** Fewest sales either period needs before a change is reported (usecase.minYoYSales). */
const MIN_YOY_SALES = 5;
const NEW_BUILDING_YEARS = 2;
const FEED_PAGE_SIZE = 50;
const MAX_PLACES = 8;
const ANALYSIS_BUCKET = 100_000;
const AGE_MIX_KEYS = ["0-5", "5-15", "15-30", "30+", "unknown"] as const;

const pad = (value: number) => String(value).padStart(2, "0");
/** Formats a UTC date, normalizing overflow as Go's AddDate does (Feb 29 + 1 year is Mar 1). */
const day = (year: number, month: number, date: number) => {
  const value = new Date(Date.UTC(year, month - 1, date));
  return `${value.getUTCFullYear()}-${pad(value.getUTCMonth() + 1)}-${pad(value.getUTCDate())}`;
};
const parts = (value: string) => value.split("-").map(Number) as [number, number, number];
const addMonths = (value: string, months: number) => { const [y, m, d] = parts(value); return day(y, m + months, d); };
const addYears = (value: string, years: number) => { const [y, m, d] = parts(value); return day(y + years, m, d); };
/** Last day of value's month. */
const monthEnd = (value: string) => { const [y, m] = parts(value); return day(y, m + 1, 0); };
const monthStart = (value: string) => `${value.slice(0, 7)}-01`;

function median(values: number[]): number {
  if (values.length === 0) return 0;
  const sorted = [...values].sort((a, b) => a - b);
  const middle = Math.floor(sorted.length / 2);
  return sorted.length % 2 ? sorted[middle] : (sorted[middle - 1] + sorted[middle]) / 2;
}

const change = (current: number, previous: number) => previous === 0 ? null : current / previous - 1;

class Group {
  count = 0;
  buildingPrice = 0;
  area = 0;
  totalPrice = 0;
  unitPrices: number[] = [];
  totalPrices: number[] = [];
  add(sale: TransactionData) {
    this.count++;
    this.buildingPrice += sale.buildingPrice;
    this.area += sale.buildingAreaPing;
    this.totalPrice += sale.totalPrice;
    this.unitPrices.push(sale.pricePerPing);
    this.totalPrices.push(sale.totalPrice);
  }
  get avgPricePerPing() { return this.area > 0 ? this.buildingPrice / this.area : 0; }
}

function grouped<K>(rows: Row[], key: (row: Row) => K): Map<K, Group> {
  const groups = new Map<K, Group>();
  for (const row of rows) {
    const value = key(row);
    let group = groups.get(value);
    if (!group) groups.set(value, group = new Group());
    group.add(row.sale);
  }
  return groups;
}

/** The value with the highest count, ties broken alphabetically (usecase.mostCommon). */
function mostCommon(counts: Map<string, number>): string {
  let best = "", bestCount = 0;
  for (const [key, count] of counts) {
    if (count > bestCount || (count === bestCount && key < best)) [best, bestCount] = [key, count];
  }
  return best;
}

function ageMixKey(age: number | null): (typeof AGE_MIX_KEYS)[number] {
  if (age === null) return "unknown";
  return age < 5 ? "0-5" : age < 15 ? "5-15" : age < 30 ? "15-30" : "30+";
}

export class LocalSales {
  private readonly rows: Row[];
  private readonly now: () => Date;

  constructor(sales: LocalSale[], private readonly options: LocalSalesOptions) {
    this.now = options.now ?? (() => new Date());
    this.rows = sales.map((sale) => {
      const saleDay = sale.transactedAt.slice(0, 10);
      const completed = sale.completedAt?.slice(0, 10);
      const age = completed && completed <= saleDay
        ? Math.floor((Date.parse(saleDay) - Date.parse(completed)) / 86_400_000 / 365.25) : null;
      return { sale, day: saleDay, month: saleDay.slice(0, 7), category: buildingCategory(sale.buildingType), age };
    }).sort((a, b) => b.day.localeCompare(a.day) || (a.sale.id < b.sale.id ? -1 : a.sale.id > b.sale.id ? 1 : 0));
  }

  /** The default window: the 12 months ending with the current one (usecase.MonthRange). */
  private window(months: number): { from: string; to: string } {
    const now = this.now();
    const current = day(now.getUTCFullYear(), now.getUTCMonth() + 1, 1);
    return { from: addMonths(current, 1 - months), to: monthEnd(current) };
  }

  /** The handler's parseFilter: the map's filters, a month or the default window. */
  private filter(query: TransactionQuery, from?: string, to?: string): Filter {
    const window = this.window(12);
    if (query.month) [from, to] = [from ?? query.month, to ?? query.month];
    return {
      minPrice: query.priceRange[0], maxPrice: query.priceRange[1], minArea: query.areaRange[0], maxArea: query.areaRange[1],
      includeSpecial: Boolean(query.includeSpecial), preciseLocation: false, excludeNewBuildings: Boolean(query.excludeNewBuildings),
      categories: query.types, bbox: query.bbox,
      from: from ? `${from}-01` : window.from, to: to ? monthEnd(`${to}-01`) : window.to,
    };
  }

  /** whereClause in the Postgres repository. */
  private matching(filter: Filter): Row[] {
    const { bbox } = filter;
    return this.rows.filter(({ sale, day: saleDay, category, age }) =>
      sale.totalPrice >= filter.minPrice && sale.totalPrice <= filter.maxPrice &&
      sale.buildingAreaPing >= filter.minArea && sale.buildingAreaPing <= filter.maxArea &&
      (filter.includeSpecial || !sale.specialReasons?.length) &&
      (!filter.preciseLocation || sale.geocodePrecision !== "district") &&
      (!filter.excludeNewBuildings || age === null || age >= NEW_BUILDING_YEARS) &&
      (filter.categories.length === 0 || filter.categories.includes(category)) &&
      (!bbox || (sale.lng >= bbox[0] && sale.lat >= bbox[1] && sale.lng <= bbox[2] && sale.lat <= bbox[3])) &&
      saleDay >= filter.from && saleDay <= filter.to);
  }

  private provisional(month: string) {
    return this.options.completeMonth !== null && month > this.options.completeMonth;
  }

  async list(query: TransactionQuery): Promise<TransactionData[]> {
    return this.matching({ ...this.filter(query), preciseLocation: true }).slice(0, 2000).map((row) => row.sale);
  }

  async aggregateH3(query: TransactionQuery, resolution: number): Promise<H3Aggregate[]> {
    const rows = this.matching({ ...this.filter(query), preciseLocation: true });
    const groups = grouped(rows, (row) => latLngToCell(row.sale.lat, row.sale.lng, resolution));
    return [...groups].sort(([a], [b]) => a < b ? -1 : 1).map(([hex, group]) => {
      const [lat, lng] = cellToLatLng(hex);
      return { hex, lat, lng, count: group.count, price: group.totalPrice / group.count, avgArea: group.area / group.count, avgPricePerPing: group.avgPricePerPing };
    });
  }

  async trends(query: TransactionQuery): Promise<TrendPoint[]> {
    const groups = grouped(this.matching(this.filter(query)), (row) => row.month);
    return [...groups].sort(([a], [b]) => a.localeCompare(b)).map(([month, group]) =>
      ({ month, avgPricePerPing: group.avgPricePerPing, count: group.count, provisional: this.provisional(month) }));
  }

  private summarize(filter: Filter) {
    const group = new Group();
    for (const row of this.matching(filter)) group.add(row.sale);
    return { count: group.count, avgPricePerPing: group.avgPricePerPing, medianTotalPrice: median(group.totalPrices) };
  }

  /** TransactionService.Summary: YoY over at least 3 months ending at the last complete month. */
  async summary(query: TransactionQuery): Promise<SummaryWithYoY> {
    const filter = this.filter(query);
    const current = this.summarize(filter);
    const [fromYear, fromMonth] = parts(filter.from), [toYear, toMonth] = parts(filter.to);
    const months = Math.max((toYear - fromYear) * 12 + toMonth - fromMonth + 1, 3);
    let end = monthStart(filter.to);
    const complete = this.options.completeMonth;
    if (complete !== null && end > `${complete}-01`) end = `${complete}-01`;

    const basis = { ...filter, from: addMonths(end, 1 - months), to: monthEnd(end) };
    const basisCurrent = basis.from === filter.from && basis.to === filter.to ? current : this.summarize(basis);
    const previous = this.summarize({ ...basis, from: addYears(basis.from, -1), to: addYears(basis.to, -1) });
    const comparable = basisCurrent.count >= MIN_YOY_SALES && previous.count >= MIN_YOY_SALES;
    return {
      ...current,
      yoy: {
        basis: `${months}m`, through: end.slice(0, 7),
        count: comparable ? change(basisCurrent.count, previous.count) : null,
        avgPricePerPing: comparable ? change(basisCurrent.avgPricePerPing, previous.avgPricePerPing) : null,
        medianTotalPrice: comparable ? change(basisCurrent.medianTotalPrice, previous.medianTotalPrice) : null,
      },
    };
  }

  async analysis(query: AnalysisQuery): Promise<MarketAnalysis> {
    const rows = this.matching(this.filter(query, query.from, query.to));
    const stats = (group: Group) => ({
      count: group.count, avgPricePerPing: group.avgPricePerPing,
      medianPricePerPing: median(group.unitPrices), medianTotalPrice: median(group.totalPrices),
    });
    const breakdown = (key: (row: Row) => string, order: string[]): BreakdownAnalysis[] => {
      const groups = grouped(rows, key);
      return order.filter((band) => groups.has(band)).map((band) => ({ key: band, ...stats(groups.get(band)!) }));
    };
    const categories = grouped(rows, (row) => row.category);
    const buckets = new Map<number, number>();
    for (const { sale } of rows) {
      const lower = Math.floor(sale.pricePerPing / ANALYSIS_BUCKET) * ANALYSIS_BUCKET;
      buckets.set(lower, (buckets.get(lower) ?? 0) + 1);
    }
    return {
      monthly: [...grouped(rows, (row) => row.month)].sort(([a], [b]) => a.localeCompare(b)).map(([month, group]) =>
        ({ month, ...stats(group), avgArea: group.area / group.count, provisional: this.provisional(month) })),
      categories: BUILDING_TYPES.filter((category) => categories.has(category)).map((category) => ({ category, ...stats(categories.get(category)!) })),
      ageGroups: breakdown(({ age }) => age === null ? "unknown" : age < 5 ? "under5" : age < 10 ? "5_9" : age < 20 ? "10_19" : age < 30 ? "20_29" : "30plus",
        ["under5", "5_9", "10_19", "20_29", "30plus", "unknown"]),
      areaGroups: breakdown(({ sale }) => sale.buildingAreaPing < 15 ? "under15" : sale.buildingAreaPing < 30 ? "15_29" : sale.buildingAreaPing < 45 ? "30_44" : "45plus",
        ["under15", "15_29", "30_44", "45plus"]),
      distribution: [...buckets].sort(([a], [b]) => a - b).map(([minPricePerPing, count]) =>
        ({ minPricePerPing, maxPricePerPing: minPricePerPing + ANALYSIS_BUCKET, count })),
    };
  }

  /** TransactionService.CompareDistricts: YoY compares the last 12 complete months with the 12 before. */
  async compareDistricts(query: DistrictQuery): Promise<DistrictComparison> {
    if (query.districts.length > 4) throw new Error("compare at most 4 districts");
    const filter = this.filter(query, query.from, query.to);
    const inCity = (rows: Row[]) => query.city === this.options.region.code ? rows : [];
    const rows = inCity(this.matching(filter));
    const ranking = [...grouped(rows, (row) => row.sale.district)].map(([district, group]) => ({
      district, count: group.count, avgPricePerPing: group.avgPricePerPing,
      medianPricePerPing: median(group.unitPrices), medianTotalPrice: median(group.totalPrices), priceYoY: null as number | null,
    })).sort((a, b) => b.medianPricePerPing - a.medianPricePerPing || (a.district < b.district ? -1 : 1));
    const complete = this.options.completeMonth;
    const yoyThrough = complete ?? "";
    const trends = [...grouped(rows.filter((row) => query.districts.includes(row.sale.district)), (row) => `${row.month}|${row.sale.district}`)]
      .sort(([a], [b]) => a < b ? -1 : 1)
      .map(([key, group]) => {
        const [month, district] = key.split("|");
        return { district, month, count: group.count, avgPricePerPing: group.avgPricePerPing, provisional: complete !== null && month > complete };
      });
    if (complete !== null) {
      const currentFrom = addMonths(`${complete}-01`, -11), previousFrom = addYears(currentFrom, -1);
      const window = inCity(this.matching({ ...filter, from: previousFrom, to: monthEnd(`${complete}-01`) }));
      const current = grouped(window.filter((row) => row.day >= currentFrom), (row) => row.sale.district);
      const previous = grouped(window.filter((row) => row.day < currentFrom), (row) => row.sale.district);
      for (const rank of ranking) {
        const now = current.get(rank.district), before = previous.get(rank.district);
        if (now && before && now.count >= MIN_YOY_SALES && before.count >= MIN_YOY_SALES && now.area > 0 && before.area > 0) {
          rank.priceYoY = change(now.avgPricePerPing, before.avgPricePerPing);
        }
      }
    }
    return { cityCode: query.city, yoyThrough, ranking, trends };
  }

  /** TransactionService.CellDetail: the hexagon's sales in the map's window, YoY over 12 complete months. */
  async cellDetail(query: TransactionQuery, hex: string): Promise<CellDetail> {
    if (!isValidCell(hex)) throw new Error(`invalid hex ${hex}`);
    const resolution = getResolution(hex);
    const filter = this.filter(query);
    let end = monthStart(filter.to);
    const complete = this.options.completeMonth;
    if (complete !== null && end > `${complete}-01`) end = `${complete}-01`;
    const currentFrom = addMonths(end, -11), currentTo = monthEnd(end), previousFrom = addYears(currentFrom, -1);
    const wide: Filter = {
      ...filter, bbox: undefined, preciseLocation: true,
      from: previousFrom < filter.from ? previousFrom : filter.from, to: currentTo > filter.to ? currentTo : filter.to,
    };

    const current = new Group(), previous = new Group(), shown = new Group();
    const districts = new Map<string, number>(), roads = new Map<string, number>(), ages = new Map<string, number>();
    const count = (counts: Map<string, number>, key: string) => counts.set(key, (counts.get(key) ?? 0) + 1);
    for (const row of this.matching(wide)) {
      if (latLngToCell(row.sale.lat, row.sale.lng, resolution) !== hex) continue;
      if (row.day >= currentFrom && row.day <= currentTo) current.add(row.sale);
      else if (row.day >= previousFrom && row.day < currentFrom) previous.add(row.sale);
      if (row.day < filter.from || row.day > filter.to) continue;
      shown.add(row.sale);
      count(districts, `${row.sale.cityCode}|${row.sale.district}`);
      if (row.sale.road) count(roads, row.sale.road);
      count(ages, ageMixKey(row.age));
    }
    const [cityCode, district] = shown.count > 0 ? mostCommon(districts).split("|") : ["", ""];
    return {
      hex, count: shown.count,
      avgPricePerPing: shown.count > 0 ? shown.avgPricePerPing : 0,
      avgTotalPrice: shown.count > 0 ? shown.totalPrice / shown.count : 0,
      avgArea: shown.count > 0 ? shown.area / shown.count : 0,
      cityCode, district, road: shown.count > 0 ? mostCommon(roads) : "",
      priceYoY: current.count >= MIN_YOY_SALES && previous.count >= MIN_YOY_SALES && current.area > 0 && previous.area > 0
        ? change(current.avgPricePerPing, previous.avgPricePerPing) : null,
      yoyThrough: end.slice(0, 7),
      ageMix: shown.count > 0 ? AGE_MIX_KEYS.map((key) => ({ key, count: ages.get(key) ?? 0 })) : [],
    };
  }

  /** FeedService.Page; the cursor is the last row's id, since the snapshot never changes. */
  async feed(query: FeedQuery, cursor?: string): Promise<FeedPage> {
    if (query.district && !query.city) throw new Error("district requires city");
    const filter = this.filter(query, query.from ?? query.month, query.to ?? query.month);
    const value = (sale: TransactionData) =>
      query.sort.startsWith("price_per_ping") ? sale.pricePerPing : query.sort === "total_price_desc" ? sale.totalPrice : sale.transactedAt;
    const direction = query.sort.endsWith("desc") ? -1 : 1;
    const rows = this.matching(filter).filter(({ sale }) =>
      (!query.city || sale.cityCode === query.city) && (!query.district || sale.district === query.district) &&
      (!query.latestOnly || sale.isNew))
      .map((row) => row.sale)
      .sort((a, b) => direction * (value(a) < value(b) ? -1 : value(a) > value(b) ? 1 : a.id < b.id ? -1 : a.id > b.id ? 1 : 0));
    const start = cursor ? rows.findIndex((sale) => sale.id === cursor) + 1 : 0;
    const page = rows.slice(start, start + FEED_PAGE_SIZE);
    return {
      data: page,
      meta: {
        total: cursor ? undefined : rows.length,
        newCount: cursor ? undefined : rows.filter((sale) => sale.isNew).length,
        nextCursor: start + FEED_PAGE_SIZE < rows.length ? page.at(-1)!.id : undefined,
        latestRelease: this.options.latestRelease,
      },
    };
  }

  async regions(): Promise<Region[]> {
    return [this.options.region];
  }

  /**
   * PlaceService.Search for the sold-to roads of one city: a district by name, or roads
   * containing the text, at the centre of their sales. Door numbers are not searched.
   */
  async searchPlaces(input: string): Promise<Place[]> {
    const { region } = this.options;
    let text = input.replace(/\s/g, "").replaceAll("臺", "台");
    if ([...text].length < 2) return [];
    text = text.startsWith(region.name) ? text.slice(region.name.length) : text;
    const district = region.districts.find((name) => text.startsWith(name)) ?? region.districts.find((name) => text === name.replace(/區$/, ""));
    if (district && (text === district || text === district.replace(/區$/, ""))) {
      const center = this.center(this.rows.filter((row) => row.sale.district === district));
      return center ? [{ kind: "district", label: region.name + district, ...center }] : [];
    }
    if (district) text = text.slice(district.length);
    const road = /^(.+?[路街道](?:[一二三四五六七八九十]+段)?)/.exec(text)?.[1] ?? text;
    const groups = new Map<string, Row[]>();
    for (const row of this.rows) {
      if (!row.sale.road.includes(road) || (district && row.sale.district !== district)) continue;
      const key = `${row.sale.district}|${row.sale.road}`;
      groups.set(key, [...(groups.get(key) ?? []), row]);
    }
    return [...groups].sort(([a, x], [b, y]) => Number(b.endsWith(`|${road}`)) - Number(a.endsWith(`|${road}`)) || y.length - x.length)
      .slice(0, MAX_PLACES)
      .map(([key, rows]) => ({ kind: "road" as const, label: region.name + key.replace("|", ""), ...this.center(rows)! }));
  }

  private center(rows: Row[]): { lat: number; lng: number } | null {
    const located = rows.filter((row) => row.sale.geocodePrecision !== "district");
    if (located.length === 0) return null;
    return {
      lat: located.reduce((sum, row) => sum + row.sale.lat, 0) / located.length,
      lng: located.reduce((sum, row) => sum + row.sale.lng, 0) / located.length,
    };
  }
}
