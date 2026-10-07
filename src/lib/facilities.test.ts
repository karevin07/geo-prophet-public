import { describe, expect, it } from "vitest";
import { categoriesOf, densityClass, groupOf, POSITIVE_CATEGORY_COUNT } from "./facilities";

describe("facility groups", () => {
  it("lists categories of the chosen groups in order", () => {
    expect(categoriesOf(["leisure", "daily"])).toEqual(["convenience", "supermarket", "market", "park", "library"]);
    expect(groupOf("substation")).toBe("nuisance");
    expect(POSITIVE_CATEGORY_COUNT).toBe(11);
  });

  it("classes density scores", () => {
    expect([1, 2, 3, 6, 8, 9, 11].map(densityClass)).toEqual([0, 0, 1, 2, 3, 4, 4]);
  });
});
