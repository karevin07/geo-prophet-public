import { describe, expect, it } from "vitest";
import { DEFAULT_STATE, formatAppState, parseAppState } from "./url-state";

describe("URL state", () => {
  it("round-trips a customised state", () => {
    const state = {
      ...DEFAULT_STATE,
      tab: "feed" as const,
      priceRange: [5_000_000, 30_000_000] as [number, number],
      areaRange: [20, 45] as [number, number],
      types: ["公寓" as const, "華廈" as const],
      includeSpecial: true,
      month: "2026-03",
      viewMode: "points" as const,
      h3Resolution: 8,
      view: { lat: 25.0375, lng: 121.5638, zoom: 15.5 },
      station: null,
      feed: { city: "a", district: "大安區", period: 3 as const, sort: "price_per_ping_desc" as const, latestOnly: true },
    };
    const url = formatAppState(state);
    expect(decodeURIComponent(url)).toBe("/feed?price=500-3000&area=20-45&types=公寓,華廈&special=1&month=2026-03&view=points&res=8&city=a&district=大安區&period=3&sort=price_per_ping_desc&latest=1&at=25.0375,121.5638,15.50");
    const [pathname, search] = url.split("?");
    expect(parseAppState(pathname, `?${search}`)).toEqual(state);
  });

  it("shares the open station on the map only", () => {
    const state = { ...DEFAULT_STATE, station: "TYMC-A14a" };
    expect(formatAppState(state)).toBe("/?station=TYMC-A14a&at=25.0400,121.5300,12.00");
    expect(parseAppState("/", "?station=TYMC-A14a").station).toBe("TYMC-A14a");
    expect(formatAppState({ ...state, tab: "feed" })).not.toContain("station");
    expect(parseAppState("/", "?station=<script>").station).toBeNull();
  });

  it("keeps default links short", () => {
    expect(formatAppState(DEFAULT_STATE)).toBe("/?at=25.0400,121.5300,12.00");
  });

  it("falls back to defaults for malformed values", () => {
    const state = parseAppState("/unknown", "?price=abc&area=50-10&types=整層住家&month=2026-3&res=12&at=x&city=taipei&district=大安區&period=5&sort=random");
    expect(state).toEqual({ ...DEFAULT_STATE, types: [] });
  });

  it("shares investment cohort filters and rejects invalid values", () => {
    const state = { ...DEFAULT_STATE, tab: "investment" as const,
      investment: { city: "f" as const, type: "公寓" as const, areaBand: 1, ageBand: 2 } };
    const url = formatAppState(state);
    expect(url).toContain("/investment?");
    expect(parseAppState("/investment", url.slice(url.indexOf("?")))).toEqual(state);
    expect(parseAppState("/investment", "?ic=x&itype=其他&ia=99&ig=-1").investment).toEqual(DEFAULT_STATE.investment);
  });

  it("shares rent filters and the compared district", () => {
    const state = { ...DEFAULT_STATE, tab: "rents" as const,
      rents: { city: "h" as const, type: "華廈" as const, areaBand: 2, includeSocial: true, district: "中壢區" } };
    const url = formatAppState(state);
    expect(url).toContain("/rents?");
    expect(parseAppState("/rents", url.slice(url.indexOf("?")))).toEqual(state);
    expect(parseAppState("/rents", "?rc=x&rtype=其他&ra=9").rents).toEqual(DEFAULT_STATE.rents);
  });

  it("shares the home being valued", () => {
    const state = { ...DEFAULT_STATE, tab: "estimate" as const,
      estimate: { address: "台北市大安區仁愛路四段27巷3號", lat: 25.038859, lng: 121.545275, type: "華廈" as const, area: 32.5, age: 53, floor: 2, totalFloors: 6, parking: true } };
    const url = formatAppState(state);
    expect(url).toContain("/estimate?");
    expect(parseAppState("/estimate", url.slice(url.indexOf("?")))).toEqual(state);
    expect(parseAppState("/estimate", "?elat=25&earea=-3&eage=x&etype=其他").estimate).toEqual(DEFAULT_STATE.estimate);
  });

  it("shares the presale filters and the open project", () => {
    const state = { ...DEFAULT_STATE, tab: "presales" as const,
      presales: { city: "a" as const, district: "文山區", months: 60 as const, sort: "price" as const, project: "a|文山區|元利四季莊園" } };
    const url = formatAppState(state);
    expect(url).toContain("/presales?");
    expect(parseAppState("/presales", url.slice(url.indexOf("?")))).toEqual(state);
    expect(parseAppState("/presales", "?pm=7&psort=x&project=bad").presales).toEqual(DEFAULT_STATE.presales);
  });

  it("shares the area overview's district", () => {
    const state = { ...DEFAULT_STATE, tab: "area" as const, area: { city: "f" as const, district: "板橋區" } };
    const url = formatAppState(state);
    expect(url).toContain("/area?");
    expect(parseAppState("/area", url.slice(url.indexOf("?")))).toEqual(state);
  });
});
