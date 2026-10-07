import { describe, expect, it } from "vitest";
import type { StationLine } from "./api";
import { smoothTrend, stationOpenings, uniqueLines, yoyColor } from "./transit";

const line = (lineId: string, openedOn: string | null, platformId = lineId): StationLine => ({
  lineId, name: lineId, color: "#000", platformId, platformName: "板橋", sequence: 1, openedOn,
});

describe("transit helpers", () => {
  it("colours rising, falling and flat or unknown changes", () => {
    // Red rising, green falling, as is usual in Taiwan.
    expect(yoyColor(0.05)[0]).toBeGreaterThan(yoyColor(0.05)[1]);
    expect(yoyColor(-0.05)[1]).toBeGreaterThan(yoyColor(-0.05)[0]);
    expect(yoyColor(0.004)).toEqual(yoyColor(null));
  });

  it("lists lines opened since 2015 once, oldest first", () => {
    const openings = stationOpenings({ lines: [line("NTMC-LB", "2026-06-30"), line("TRTC-BL", null), line("NTMC-Y", "2020-01-31"), line("NTMC-Y", "2020-01-31", "NTMC-Y2")] });
    expect(openings.map((opening) => `${opening.lineId} ${opening.month}`)).toEqual(["NTMC-Y 2020-01", "NTMC-LB 2026-06"]);
    expect(uniqueLines([line("TRA-WL", null, "TRA-1"), line("TRA-WL", null, "TRA-2")])).toHaveLength(1);
  });

  it("fills missing months and smooths by sales over three months", () => {
    const points = smoothTrend([
      { month: "2025-11", avgPricePerPing: 600_000, count: 1, provisional: false },
      { month: "2026-01", avgPricePerPing: 900_000, count: 2, provisional: true },
    ]);
    expect(points.map((point) => point.month)).toEqual(["2025-11", "2025-12", "2026-01"]);
    // Too few sales until the third month's window holds three.
    expect(points[1]).toMatchObject({ price: null, count: 0 });
    expect(points[2]).toMatchObject({ price: 80, count: 2, provisional: true });
  });
});
