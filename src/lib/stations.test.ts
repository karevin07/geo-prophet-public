import { describe, expect, it } from "vitest";
import type { OpeningEffect, StationRank, TransitStation } from "./api";
import { distanceRows, matchesStation, nearStationPremium, openingChartRows, openingRows, rankRows, sparkline } from "./stations";

const station = (id: string, name: string, patch: Partial<TransitStation> = {}): TransitStation => ({
  id, name, nameEn: "", cityCode: "f", district: "板橋區", lat: 25, lng: 121.5, systems: ["metro"], openedOn: null,
  lines: [{ lineId: "TRTC-BL", name: "板南線", color: "#0a59ae", platformId: id, platformName: name, sequence: 1, openedOn: null }], ...patch,
});
const rank = (stationId: string, medianPricePerPing: number, priceYoY: number | null, count = 10): StationRank => ({
  stationId, count, avgPricePerPing: medianPricePerPing, medianPricePerPing, priceYoY, trend: [],
});
const all = { city: "", systems: [], lineId: "" };

describe("station helpers", () => {
  const banqiao = station("TRTC-BL07", "板橋", { systems: ["metro", "tra", "thsr"] });
  const daan = station("TRTC-R05", "大安", { cityCode: "a", lines: [{ lineId: "TRTC-R", name: "淡水信義線", color: "#d90023", platformId: "TRTC-R05", platformName: "大安", sequence: 5, openedOn: null }] });
  const quiet = station("TRA-7336", "菁桐", { systems: ["tra"] });

  it("filters by city, system and line", () => {
    expect(matchesStation(banqiao, { ...all, systems: ["thsr"] })).toBe(true);
    expect(matchesStation(daan, { ...all, city: "f" })).toBe(false);
    expect(matchesStation(daan, { ...all, lineId: "TRTC-BL" })).toBe(false);
  });

  it("ranks stations with sales, putting unknown YoY last", () => {
    const ranks = [rank("TRTC-BL07", 734_000, 0.11), rank("TRTC-R05", 1_286_000, null), rank("TRA-7336", 200_000, 0.2, 0)];
    const stations = [banqiao, daan, quiet];
    expect(rankRows(stations, ranks, all, "median").map((row) => row.station.name)).toEqual(["大安", "板橋"]);
    expect(rankRows(stations, ranks, all, "yoy").map((row) => row.station.name)).toEqual(["板橋", "大安"]);
  });

  it("orders openings by comparability and relative change", () => {
    const effect = (stationId: string, status: OpeningEffect["status"], relative: number | null) => ({ stationId, status, relative, openedOn: "2020-01-31" }) as OpeningEffect;
    const rows = openingRows([effect("TRA-7336", "insufficient", null), effect("TRTC-R05", "ok", 0.02), effect("TRTC-BL07", "ok", 0.06)], [banqiao, daan, quiet], all);
    expect(rows.map((row) => row.stationId)).toEqual(["TRTC-BL07", "TRTC-R05", "TRA-7336"]);
  });

  it("draws sparklines and smooths opening series", () => {
    expect(sparkline([{ month: "2026-01", avgPricePerPing: 1, count: 1, provisional: false }])).toBe("");
    expect(sparkline([
      { month: "2026-01", avgPricePerPing: 100, count: 1, provisional: false },
      { month: "2026-02", avgPricePerPing: 200, count: 1, provisional: false },
    ], 10, 10)).toBe("0.0,8.0 10.0,2.0");
    const rows = openingChartRows({
      series: [
        { month: "2019-12", offset: -1, stationPrice: 500_000, stationCount: 3, baselinePrice: 400_000, baselineCount: 50 },
        { month: "2020-01", offset: 0, stationPrice: null, stationCount: 0, baselinePrice: 410_000, baselineCount: 50 },
      ],
    } as OpeningEffect);
    expect(rows.map((row) => [row.offset, row.station, row.baseline])).toEqual([[-1, 50, 40], [0, 50, 40.5]]);
  });

  it("pivots distance buckets and compares the nearest with 700–800 m", () => {
    const bucket = (cityCode: string, fromM: number, count: number, median: number) => ({
      cityCode, fromM, toM: fromM === 800 ? null : fromM + 100, count, avgPricePerPing: median, medianPricePerPing: median,
    });
    const buckets = [bucket("a", 0, 284, 974_000), bucket("a", 700, 201, 676_000), bucket("a", 800, 976, 623_000), bucket("h", 0, 14, 481_000), bucket("h", 700, 153, 363_000)];
    const rows = distanceRows(buckets, ["a"]);
    expect(rows.map((row) => row.label)).toEqual(["0–100", "700–800", "800+"]);
    expect(rows[0].median).toEqual({ a: 97.4 });
    expect(nearStationPremium(buckets, "a")?.toFixed(3)).toBe("0.441");
    expect(nearStationPremium(buckets, "h")).toBeNull(); // 14 sales near stations
  });
});
