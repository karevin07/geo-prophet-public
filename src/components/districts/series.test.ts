import { describe, expect, it } from "vitest";
import { calendarMonths, districtChartRows } from "./series";

describe("district comparison series", () => {
  it("keeps empty months and distinguishes missing price from zero volume", () => {
    const months = calendarMonths("2026-01", "2026-03");
    const trends = [{ district: "大安區", month: "2026-02", count: 2, avgPricePerPing: 800_000, provisional: false }];
    expect(months).toEqual(["2026-01", "2026-02", "2026-03"]);
    expect(districtChartRows(trends, ["大安區"], "price", months)).toEqual([
      { month: "2026-01", 大安區: null }, { month: "2026-02", 大安區: 80 }, { month: "2026-03", 大安區: null },
    ]);
    expect(districtChartRows(trends, ["大安區"], "volume", months)).toEqual([
      { month: "2026-01", 大安區: 0 }, { month: "2026-02", 大安區: 2 }, { month: "2026-03", 大安區: 0 },
    ]);
  });
});
