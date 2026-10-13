import { BUILDING_TYPES, FEED_SORTS, type BuildingType, type FeedSort } from "./api/types";
import { tabAvailable } from "./demo";

export const TABS = ["map", "analysis", "districts", "area", "investment", "rents", "estimate", "presales", "feed", "stations", "community"] as const;
export type Tab = (typeof TABS)[number];

const TAB_PATHS: Record<Tab, string> = { map: "/", analysis: "/analysis", districts: "/districts", area: "/area", investment: "/investment", rents: "/rents", estimate: "/estimate", presales: "/presales", feed: "/feed", stations: "/stations", community: "/community" };

/** Where the app is served, without the trailing slash: "" at the root, "/geo-prophet-public" for the demo. */
export const BASE_PATH = import.meta.env.BASE_URL.replace(/\/+$/, "");

/** A tab's address, under the app's base path. */
export function tabPath(tab: Tab): string {
  return tab === "map" ? `${BASE_PATH}/` : `${BASE_PATH}${TAB_PATHS[tab]}`;
}

export const INVESTMENT_AREA_BANDS = [[15, 30], [30, 45], [45, 60], [15, 45]] as const;
export const INVESTMENT_AGE_BANDS = [[0, 5], [5, 15], [15, 30], [30, 60], [5, 30]] as const;
export interface InvestmentState { city: "a" | "f" | "h"; type: "住宅大樓" | "華廈" | "公寓" | "透天厝"; areaBand: number; ageBand: number }

/** Rent page area bands in ping; [0, 0] is every size. */
export const RENT_AREA_BANDS = [[0, 0], [15, 25], [25, 35], [35, 50]] as const;
export const RENT_TYPES = ["", "住宅大樓", "華廈", "公寓", "透天厝"] as const;
export interface RentsState {
  city: "a" | "f" | "h";
  /** "" is every residential type. */
  type: (typeof RENT_TYPES)[number];
  areaBand: number;
  includeSocial: boolean;
  /** The district whose trend is drawn next to the city's. */
  district: string;
}

export const ESTIMATE_TYPES = ["住宅大樓", "華廈", "公寓", "透天厝"] as const;
/** The home being valued; null and 0 are unknown. */
export interface EstimateState {
  address: string;
  lat: number | null;
  lng: number | null;
  type: (typeof ESTIMATE_TYPES)[number];
  area: number | null;
  age: number | null;
  floor: number;
  totalFloors: number;
  parking: boolean;
}

export const PRESALE_MONTHS = [12, 24, 60] as const;
export const PRESALE_SORTS = ["recent", "sales", "price"] as const;
export interface PresalesState {
  city: "a" | "f" | "h";
  district: string;
  months: (typeof PRESALE_MONTHS)[number];
  sort: (typeof PRESALE_SORTS)[number];
  /** The open project's key, city|district|name. */
  project: string;
}

/** The district of the area overview; "" picks the busiest one. */
export interface AreaState {
  city: "a" | "f" | "h";
  district: string;
}

/** Feed periods in months back from the current month; 0 is every month on record. */
export const FEED_PERIODS = [1, 3, 12, 36, 0] as const;
export type FeedPeriod = (typeof FEED_PERIODS)[number];

export interface FeedState {
  city: string;
  district: string;
  period: FeedPeriod;
  sort: FeedSort;
  latestOnly: boolean;
}

export interface MapView {
  lat: number;
  lng: number;
  zoom: number;
}

/** Everything a shared link restores. Prices are TWD; the URL stores them in 萬. */
export interface AppState {
  tab: Tab;
  priceRange: [number, number];
  areaRange: [number, number];
  types: BuildingType[];
  includeSpecial: boolean;
  month: string | null;
  viewMode: "hex" | "points";
  h3Resolution: number;
  view: MapView;
  /** The station whose card is open on the map, e.g. TRTC-BL07. */
  station: string | null;
  feed: FeedState;
  investment: InvestmentState;
  rents: RentsState;
  estimate: EstimateState;
  presales: PresalesState;
  area: AreaState;
  /** The open community's ID, e.g. 3d2801a712dc. */
  community: string;
}

export const DEFAULT_STATE: AppState = {
  tab: "map",
  priceRange: [0, 100_000_000],
  areaRange: [0, 200],
  types: [...BUILDING_TYPES],
  includeSpecial: false,
  month: null,
  viewMode: "hex",
  h3Resolution: 9,
  view: { lat: 25.04, lng: 121.53, zoom: 12 },
  station: null,
  feed: { city: "", district: "", period: 12, sort: "date_desc", latestOnly: false },
  investment: { city: "a", type: "住宅大樓", areaBand: 3, ageBand: 4 },
  rents: { city: "a", type: "", areaBand: 0, includeSocial: false, district: "" },
  estimate: { address: "", lat: null, lng: null, type: "住宅大樓", area: null, age: null, floor: 0, totalFloors: 0, parking: false },
  presales: { city: "a", district: "", months: 24, sort: "sales", project: "" },
  area: { city: "a", district: "" },
  community: "",
};

/** A non-negative number from a URL parameter within max, or null. */
function numberParam(params: URLSearchParams, key: string, max: number): number | null {
  const value = params.get(key);
  if (value === null || value === "") return null;
  const number = Number(value);
  return Number.isFinite(number) && number >= 0 && number <= max ? number : null;
}

const WAN = 10_000;

function parseRange(value: string | null, scale: number, fallback: [number, number]): [number, number] {
  const match = value?.match(/^(\d+(?:\.\d+)?)-(\d+(?:\.\d+)?)$/);
  if (!match) return fallback;
  const low = Number(match[1]) * scale;
  const high = Number(match[2]) * scale;
  return low <= high ? [low, high] : fallback;
}

function clamp(value: number, min: number, max: number) {
  return Math.min(Math.max(value, min), max);
}

export function parseAppState(pathname: string, search: string): AppState {
  const params = new URLSearchParams(search);
  const path = (pathname.startsWith(BASE_PATH) ? pathname.slice(BASE_PATH.length) : pathname).replace(/\/+$/, "") || "/";
  const named = (Object.entries(TAB_PATHS).find(([, tabPath]) => tabPath === path)?.[0] ?? "map") as Tab;
  const community = /^[0-9a-f]{12}$/.test(params.get("id") ?? "") ? params.get("id")! : "";
  // A community page needs its ID; without one, the map opens.
  const tab = tabAvailable(named) && (named !== "community" || community) ? named : "map";
  const types = params.get("types")?.split(",").filter((type): type is BuildingType => (BUILDING_TYPES as readonly string[]).includes(type));
  const month = params.get("month");
  const resolution = Number(params.get("res"));
  const [lat, lng, zoom] = (params.get("at") ?? "").split(",").map(Number);
  const period = Number(params.get("period"));
  const sort = params.get("sort");
  const city = params.get("city") ?? "";
  const station = params.get("station");
  const investmentCity = params.get("ic");
  const investmentType = params.get("itype");
  const investmentArea = Number(params.get("ia"));
  const investmentAge = Number(params.get("ig"));
  const rentCity = params.get("rc");
  const rentType = params.get("rtype") ?? "";
  const rentArea = Number(params.get("ra"));

  return {
    tab,
    priceRange: parseRange(params.get("price"), WAN, DEFAULT_STATE.priceRange),
    areaRange: parseRange(params.get("area"), 1, DEFAULT_STATE.areaRange),
    types: types && params.has("types") ? types : DEFAULT_STATE.types,
    includeSpecial: params.get("special") === "1",
    month: month && /^\d{4}-\d{2}$/.test(month) ? month : null,
    viewMode: params.get("view") === "points" ? "points" : "hex",
    h3Resolution: Number.isInteger(resolution) && resolution >= 7 && resolution <= 10 ? resolution : DEFAULT_STATE.h3Resolution,
    view: Number.isFinite(lat) && Number.isFinite(lng) && Number.isFinite(zoom) && lat !== 0
      ? { lat: clamp(lat, -85, 85), lng: clamp(lng, -180, 180), zoom: clamp(zoom, 3, 20) }
      : DEFAULT_STATE.view,
    // Station IDs are operator-StationID, e.g. TRTC-BL07 or TYMC-A14a.
    station: station && /^[A-Z]+-[0-9A-Za-z]+$/.test(station) ? station : null,
    feed: {
      city: /^[a-z]$/.test(city) ? city : "",
      district: /^[a-z]$/.test(city) ? params.get("district") ?? "" : "",
      period: params.has("period") && (FEED_PERIODS as readonly number[]).includes(period) ? period as FeedPeriod : DEFAULT_STATE.feed.period,
      sort: (FEED_SORTS as readonly string[]).includes(sort ?? "") ? sort as FeedSort : DEFAULT_STATE.feed.sort,
      latestOnly: params.get("latest") === "1",
    },
    investment: {
      city: investmentCity === "f" || investmentCity === "h" ? investmentCity : "a",
      type: investmentType === "華廈" || investmentType === "公寓" || investmentType === "透天厝" ? investmentType : "住宅大樓",
      areaBand: params.has("ia") && Number.isInteger(investmentArea) && investmentArea >= 0 && investmentArea < INVESTMENT_AREA_BANDS.length ? investmentArea : DEFAULT_STATE.investment.areaBand,
      ageBand: params.has("ig") && Number.isInteger(investmentAge) && investmentAge >= 0 && investmentAge < INVESTMENT_AGE_BANDS.length ? investmentAge : DEFAULT_STATE.investment.ageBand,
    },
    estimate: parseEstimate(params),
    presales: parsePresales(params),
    area: {
      city: params.get("ac") === "f" || params.get("ac") === "h" ? params.get("ac") as AreaState["city"] : "a",
      district: params.get("ad") ?? "",
    },
    rents: {
      city: rentCity === "f" || rentCity === "h" ? rentCity : "a",
      type: (RENT_TYPES as readonly string[]).includes(rentType) ? rentType as RentsState["type"] : "",
      areaBand: Number.isInteger(rentArea) && rentArea > 0 && rentArea < RENT_AREA_BANDS.length ? rentArea : 0,
      includeSocial: params.get("rs") === "1",
      district: params.get("rd") ?? "",
    },
    community,
  };
}

function parsePresales(params: URLSearchParams): PresalesState {
  const city = params.get("pc");
  const months = Number(params.get("pm"));
  const sort = params.get("psort") ?? "";
  const project = params.get("project") ?? "";
  return {
    city: city === "f" || city === "h" ? city : "a",
    district: params.get("pd") ?? "",
    months: (PRESALE_MONTHS as readonly number[]).includes(months) ? months as PresalesState["months"] : DEFAULT_STATE.presales.months,
    sort: (PRESALE_SORTS as readonly string[]).includes(sort) ? sort as PresalesState["sort"] : DEFAULT_STATE.presales.sort,
    project: /^[a-z]\|[^|]+\|.+$/.test(project) ? project : "",
  };
}

function parseEstimate(params: URLSearchParams): EstimateState {
  const lat = numberParam(params, "elat", 90);
  const lng = numberParam(params, "elng", 180);
  const type = params.get("etype") ?? "";
  const located = lat !== null && lng !== null && params.has("eaddr");
  return {
    address: located ? params.get("eaddr") ?? "" : "",
    lat: located ? lat : null,
    lng: located ? lng : null,
    type: (ESTIMATE_TYPES as readonly string[]).includes(type) ? type as EstimateState["type"] : DEFAULT_STATE.estimate.type,
    area: numberParam(params, "earea", 300),
    age: numberParam(params, "eage", 100),
    floor: Math.round(numberParam(params, "efloor", 120) ?? 0),
    totalFloors: Math.round(numberParam(params, "efloors", 120) ?? 0),
    parking: params.get("epark") === "1",
  };
}

/** Serializes only non-default values so shared links stay short. */
export function formatAppState(state: AppState): string {
  const params = new URLSearchParams();
  const range = ([low, high]: [number, number], scale: number) => `${low / scale}-${high / scale}`;
  if (state.priceRange.join() !== DEFAULT_STATE.priceRange.join()) params.set("price", range(state.priceRange, WAN));
  if (state.areaRange.join() !== DEFAULT_STATE.areaRange.join()) params.set("area", range(state.areaRange, 1));
  if (state.types.length !== BUILDING_TYPES.length) params.set("types", state.types.join(","));
  if (state.includeSpecial) params.set("special", "1");
  if (state.month) params.set("month", state.month);
  if (state.viewMode !== "hex") params.set("view", state.viewMode);
  if (state.h3Resolution !== DEFAULT_STATE.h3Resolution) params.set("res", String(state.h3Resolution));
  if (state.station && state.tab === "map") params.set("station", state.station);
  const { feed } = state;
  if (feed.city) params.set("city", feed.city);
  if (feed.city && feed.district) params.set("district", feed.district);
  if (feed.period !== DEFAULT_STATE.feed.period) params.set("period", String(feed.period));
  if (feed.sort !== DEFAULT_STATE.feed.sort) params.set("sort", feed.sort);
  if (feed.latestOnly) params.set("latest", "1");
  if (state.investment.city !== DEFAULT_STATE.investment.city) params.set("ic", state.investment.city);
  if (state.investment.type !== DEFAULT_STATE.investment.type) params.set("itype", state.investment.type);
  if (state.investment.areaBand !== DEFAULT_STATE.investment.areaBand) params.set("ia", String(state.investment.areaBand));
  if (state.investment.ageBand !== DEFAULT_STATE.investment.ageBand) params.set("ig", String(state.investment.ageBand));
  if (state.area.city !== DEFAULT_STATE.area.city) params.set("ac", state.area.city);
  if (state.area.district) params.set("ad", state.area.district);
  const { presales } = state;
  if (presales.city !== DEFAULT_STATE.presales.city) params.set("pc", presales.city);
  if (presales.district) params.set("pd", presales.district);
  if (presales.months !== DEFAULT_STATE.presales.months) params.set("pm", String(presales.months));
  if (presales.sort !== DEFAULT_STATE.presales.sort) params.set("psort", presales.sort);
  if (presales.project) params.set("project", presales.project);
  const { estimate } = state;
  if (estimate.address && estimate.lat !== null && estimate.lng !== null) {
    params.set("eaddr", estimate.address);
    params.set("elat", estimate.lat.toFixed(6));
    params.set("elng", estimate.lng.toFixed(6));
  }
  if (estimate.type !== DEFAULT_STATE.estimate.type) params.set("etype", estimate.type);
  if (estimate.area !== null) params.set("earea", String(estimate.area));
  if (estimate.age !== null) params.set("eage", String(estimate.age));
  if (estimate.floor) params.set("efloor", String(estimate.floor));
  if (estimate.totalFloors) params.set("efloors", String(estimate.totalFloors));
  if (estimate.parking) params.set("epark", "1");
  const { rents } = state;
  if (rents.city !== DEFAULT_STATE.rents.city) params.set("rc", rents.city);
  if (rents.type) params.set("rtype", rents.type);
  if (rents.areaBand) params.set("ra", String(rents.areaBand));
  if (rents.includeSocial) params.set("rs", "1");
  if (rents.district) params.set("rd", rents.district);
  if (state.tab === "community" && state.community) params.set("id", state.community);
  const { lat, lng, zoom } = state.view;
  params.set("at", `${lat.toFixed(4)},${lng.toFixed(4)},${zoom.toFixed(2)}`);
  const query = params.toString().replace(/%2C/g, ",");
  return `${tabPath(state.tab)}${query ? `?${query}` : ""}`;
}
