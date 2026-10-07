import type { DistrictTrend } from "../../lib/api";

export { calendarMonths } from "../../lib/utils";

export type DistrictMetric = "price" | "volume";

export function districtChartRows(trends: DistrictTrend[], districts: string[], metric: DistrictMetric, months: string[]) {
  const points = new Map(trends.map((point) => [`${point.district}|${point.month}`, point]));
  return months.map((month) => {
    const row: Record<string, string | number | null> = { month };
    for (const district of districts) {
      const point = points.get(`${district}|${month}`);
      row[district] = metric === "volume" ? point?.count ?? 0 : point ? point.avgPricePerPing / 10_000 : null;
    }
    return row;
  });
}
