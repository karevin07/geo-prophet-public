import { describe, expect, it } from "vitest";
import { formatYield, rentRows, yieldBreaks } from "./rent";

describe("yieldBreaks", () => {
  it("rounds quartiles to 0.1 percentage point and ignores missing yields", () => {
    expect(yieldBreaks([0.0193, null, 0.0201, 0.0232, 0.0249, 0.0254, 0.0295])).toEqual([0.021, 0.024, 0.025]);
    expect(yieldBreaks([null])).toBeNull();
  });
});

describe("formatYield", () => {
  it("formats a ratio as a percentage", () => {
    expect(formatYield(0.02347)).toBe("2.35%");
    expect(formatYield(null)).toBe("—");
  });
});

describe("rentRows", () => {
  it("dashes the provisional tail from the last settled month", () => {
    const rows = rentRows([
      { month: "2026-04", rentals: 300, medianRentPerPing: 1500, provisional: false },
      { month: "2026-05", rentals: 310, medianRentPerPing: 1510, provisional: false },
      { month: "2026-06", rentals: 120, medianRentPerPing: 1530, provisional: true },
    ], [{ month: "2026-05", rentals: 40, medianRentPerPing: 2000, provisional: false }]);
    expect(rows.map((row) => [row.month, row.city, row.cityTail, row.district])).toEqual([
      ["2026-04", 1500, null, null],
      ["2026-05", 1510, 1510, 2000],
      ["2026-06", null, 1530, null],
    ]);
  });
});
