import type { ImportDataset, ImportRun, SeasonCoverage } from "./api";

/** Rows of the dataset matrix, in pipeline order. */
export const DATASETS: ImportDataset[] = ["sales", "sales_release", "rentals", "rentals_release", "presales", "presales_release", "door_numbers", "district_boundaries", "transit", "osm_facilities", "nhi_facilities", "moe_schools"];
/** Datasets imported once for every city rather than per city. */
export const SHARED_DATASETS: ImportDataset[] = ["transit", "osm_facilities", "nhi_facilities", "moe_schools"];
export const CITIES = ["a", "f", "h"] as const;
export const CITY_NAMES: Record<string, string> = { a: "台北市", f: "新北市", h: "桃園市" };

/** 實價登錄 began in 101S1 (August 2012 falls in its second season). */
const FIRST_SEASON = "101S1";

export type SeasonState = "imported" | "missing" | "pending" | "before";

interface Season {
  year: number;
  quarter: number;
}

function parseSeason(label: string): Season | null {
  const match = /^(\d{3})S([1-4])$/.exec(label);
  return match ? { year: Number(match[1]), quarter: Number(match[2]) } : null;
}

function formatSeason({ year, quarter }: Season) {
  return `${String(year).padStart(3, "0")}S${quarter}`;
}

function nextSeason({ year, quarter }: Season): Season {
  return quarter === 4 ? { year: year + 1, quarter: 1 } : { year, quarter: quarter + 1 };
}

/**
 * The season whose registrations are being collected on the given day. Seasons follow
 * registration dates: S1 12/11–3/10, S2 3/11–6/10, S3 6/11–9/10, S4 9/11–12/10.
 */
export function seasonInProgress(day: Date): string {
  const year = day.getFullYear() - 1911;
  const monthDay = (day.getMonth() + 1) * 100 + day.getDate();
  if (monthDay >= 1211) return formatSeason({ year: year + 1, quarter: 1 });
  if (monthDay >= 911) return formatSeason({ year, quarter: 4 });
  if (monthDay >= 611) return formatSeason({ year, quarter: 3 });
  if (monthDay >= 311) return formatSeason({ year, quarter: 2 });
  return formatSeason({ year, quarter: 1 });
}

export interface SeasonCell {
  season: string;
  state: SeasonState;
}

/**
 * Every season from 101S1 to the one in progress, marked imported, missing (a gap between
 * imported seasons), pending (after the latest import: not yet published or not yet
 * imported) or before (earlier than the first import).
 */
export function seasonCells(coverage: SeasonCoverage | undefined, today: Date): SeasonCell[] {
  const first = coverage && parseSeason(coverage.firstImported);
  const latest = coverage && parseSeason(coverage.latestImported);
  const missing = new Set(coverage?.missing ?? []);
  const end = seasonInProgress(today);
  const cells: SeasonCell[] = [];
  for (let season = parseSeason(FIRST_SEASON)!; ; season = nextSeason(season)) {
    const label = formatSeason(season);
    let state: SeasonState = "pending";
    if (first && latest) {
      if (label < formatSeason(first)) state = "before";
      else if (label <= formatSeason(latest)) state = missing.has(label) ? "missing" : "imported";
    }
    cells.push({ season: label, state });
    if (label >= end) return cells;
  }
}

export interface DatasetCell {
  latest?: ImportRun;
  lastSuccess?: ImportRun;
}

/** The latest attempt and latest success of each dataset, by city ("" for shared ones). */
export function datasetMatrix(latestRuns: ImportRun[], lastSuccessfulRuns: ImportRun[]): Map<string, DatasetCell> {
  const matrix = new Map<string, DatasetCell>();
  const key = (run: ImportRun) => `${run.dataset}|${run.cityCode}`;
  for (const run of latestRuns) matrix.set(key(run), { ...matrix.get(key(run)), latest: run });
  for (const run of lastSuccessfulRuns) matrix.set(key(run), { ...matrix.get(key(run)), lastSuccess: run });
  return matrix;
}

/** 1150901-1150910 → 115/09/01–09/10; other labels are returned unchanged. */
export function periodLabel(label: string): string {
  const match = /^(\d{3})(\d{2})(\d{2})-\d{3}(\d{2})(\d{2})$/.exec(label);
  return match ? `${match[1]}/${match[2]}/${match[3]}–${match[4]}/${match[5]}` : label;
}

/** Run time as 1.6s, 42s or 6m20s; null while running. */
export function runDuration(run: ImportRun): string | null {
  if (!run.finishedAt) return null;
  const seconds = (Date.parse(run.finishedAt) - Date.parse(run.startedAt)) / 1000;
  if (seconds < 10) return `${seconds.toFixed(1)}s`;
  if (seconds < 60) return `${Math.round(seconds)}s`;
  const minutes = Math.floor(seconds / 60);
  return `${minutes}m${String(Math.round(seconds - minutes * 60)).padStart(2, "0")}s`;
}
