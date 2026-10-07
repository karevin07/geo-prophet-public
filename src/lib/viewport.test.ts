import { describe, expect, it } from "vitest";
import { roundBBox, sameBBox } from "./viewport";

describe("roundBBox", () => {
  it("rounds to three decimals and clamps latitude", () => {
    expect(roundBBox([121.45678, 24.98712, 121.60049, 89.5])).toEqual([121.457, 24.987, 121.6, 85]);
  });
});

describe("sameBBox", () => {
  it("compares by value", () => {
    expect(sameBBox([1, 2, 3, 4], [1, 2, 3, 4])).toBe(true);
    expect(sameBBox([1, 2, 3, 4], [1, 2, 3, 5])).toBe(false);
    expect(sameBBox(null, [1, 2, 3, 4])).toBe(false);
  });
});
