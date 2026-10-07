import { describe, expect, it } from "vitest";
import { floorNumber } from "./floors";

describe("floorNumber", () => {
  it("reads Chinese and Arabic floors", () => {
    expect(floorNumber("二十三層")).toBe(23);
    expect(floorNumber("十層,陽台")).toBe(10);
    expect(floorNumber("一百零一層")).toBe(101);
    expect(floorNumber("4層")).toBe(4);
    expect(floorNumber("地下二層")).toBeNull();
    expect(floorNumber("全")).toBeNull();
  });
});
