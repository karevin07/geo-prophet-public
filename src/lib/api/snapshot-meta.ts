import type { Region, Release } from "./types";

/** meta.json of a snapshot written by api/cmd/snapshot. */
export interface SnapshotMeta {
  version: number;
  generatedAt: string;
  cityCode: string;
  /** First day of sales kept and the latest sale (YYYY-MM-DD). */
  from: string;
  through: string;
  completeMonth: string;
  latestRelease: Release | null;
  region: Region;
  sales: number;
}

export async function readJSON<T>(url: string, signal?: AbortSignal): Promise<T> {
  const response = await fetch(url, { signal });
  if (!response.ok) throw new Error(`${url}: ${response.status}`);
  return response.json() as Promise<T>;
}

export function readSnapshotMeta(base: string, signal?: AbortSignal): Promise<SnapshotMeta> {
  return readJSON<SnapshotMeta>(`${base}meta.json`, signal);
}
