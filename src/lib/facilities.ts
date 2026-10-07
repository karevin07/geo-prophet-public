import type { FacilityGroup } from "./api";

export const FACILITY_GROUPS: FacilityGroup[] = ["daily", "education", "medical", "leisure", "nuisance"];

/** Categories of each group, in display order (the API's order). */
export const FACILITY_CATEGORIES: Record<FacilityGroup, string[]> = {
  daily: ["convenience", "supermarket", "market"],
  education: ["elementary", "junior_high", "senior_high", "university"],
  medical: ["hospital", "clinic"],
  leisure: ["park", "library"],
  nuisance: ["gas_station", "substation", "funeral", "cemetery", "waste"],
};

/** Positive categories: everything but nuisances. Density scores range up to this. */
export const POSITIVE_CATEGORY_COUNT = FACILITY_GROUPS.filter((group) => group !== "nuisance")
  .reduce((sum, group) => sum + FACILITY_CATEGORIES[group].length, 0);

export const GROUP_COLORS: Record<FacilityGroup, string> = {
  daily: "#1F6F78",
  education: "#4338CA",
  medical: "#DC2626",
  leisure: "#16A34A",
  nuisance: "#374151",
};

export function groupOf(category: string): FacilityGroup | undefined {
  return FACILITY_GROUPS.find((group) => FACILITY_CATEGORIES[group].includes(category));
}

export function categoriesOf(groups: FacilityGroup[]): string[] {
  return FACILITY_GROUPS.filter((group) => groups.includes(group)).flatMap((group) => FACILITY_CATEGORIES[group]);
}

export function rgb(hex: string, alpha = 255): [number, number, number, number] {
  const value = parseInt(hex.slice(1), 16);
  return [(value >> 16) & 255, (value >> 8) & 255, value & 255, alpha];
}

/** Density classes by the number of kinds of facilities nearby: 1–2, 3–4, 5–6, 7–8, 9+. */
export const DENSITY_CLASSES = [
  { min: 1, label: "1–2", hex: "#E6F2F3" },
  { min: 3, label: "3–4", hex: "#9CCBD0" },
  { min: 5, label: "5–6", hex: "#72B7BC" },
  { min: 7, label: "7–8", hex: "#4FA3A5" },
  { min: 9, label: `9–${POSITIVE_CATEGORY_COUNT}`, hex: "#1F6F78" },
];

export function densityClass(score: number): number {
  let index = 0;
  DENSITY_CLASSES.forEach((entry, position) => {
    if (score >= entry.min) index = position;
  });
  return index;
}
