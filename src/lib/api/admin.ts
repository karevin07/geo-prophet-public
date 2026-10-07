import type { DataStatus } from "./types";

/** The Tailscale user behind the request; login is empty without `tailscale serve`. */
export interface AdminSession {
  login: string;
  name: string;
  profilePic: string;
  admin: boolean;
}

export interface TableSize {
  name: string;
  bytes: number;
}

export interface BackupFile {
  name: string;
  bytes: number;
  modifiedAt: string;
}

export interface SystemStatus {
  generatedAt: string;
  database: { bytes: number; transactions: number; tables: TableSize[] };
  /** The filesystem holding the backups; null when they are not mounted into the API. */
  disk: { totalBytes: number; usedBytes: number; freeBytes: number } | null;
  /** Newest first. */
  backups: BackupFile[];
  backupKeep: number;
  backupIntervalHours: number;
}

export type JobKind = "sync" | "current_release" | "transit" | "backup" | "seasons";
/** Kinds run with one click; seasons jobs need input and get their own card. */
export const JOB_KINDS: JobKind[] = ["sync", "current_release", "transit", "backup"];

/** A seasons job's input: 115S2, 114S1,114S2 or 105S1-115S2, for some cities. */
export interface SeasonsParams {
  seasons: string;
  cities: string[];
}
export type JobStatus = "queued" | "running" | "succeeded" | "failed";

/** A run requested from the console; the importer or backup service picks it up. */
export interface AdminJob {
  id: number;
  kind: JobKind;
  status: JobStatus;
  /** Tailscale login of whoever asked. */
  requestedBy: string;
  requestedAt: string;
  /** Input of jobs that take one; {} otherwise. */
  params: Partial<SeasonsParams>;
  startedAt: string | null;
  finishedAt: string | null;
  /** {runs, failedRuns, rowsUpserted} for imports, {file, bytes} for backups; {} until done. */
  result: { runs?: number; failedRuns?: number; rowsUpserted?: number; file?: string; bytes?: number };
  error?: string;
}

export interface LineOpening {
  lineId: string;
  name: string;
  system: string;
  color: string;
  /** YYYY-MM-DD; null when the line opened before 2015. */
  openedOn: string | null;
  source: string;
  updatedBy: string;
  updatedAt: string | null;
}

export interface PlatformOpening {
  platformId: string;
  platformName: string;
  stationName: string;
  lines: string;
  lineColor: string;
  lineOpenedOn: string | null;
  openedOn: string;
  source: string;
  updatedBy: string;
  updatedAt: string;
}

export interface PlatformOption {
  platformId: string;
  platformName: string;
  stationName: string;
  lines: string;
}

export interface Openings {
  lines: LineOpening[];
  /** Platforms with their own date, i.e. opened after their line. */
  platforms: PlatformOpening[];
  /** Every platform, for adding one. */
  options: PlatformOption[];
}

export interface OpeningEdit {
  kind: "line" | "platform";
  targetId: string;
  openedOn: string | null;
  source: string;
}

/** An admin API failure, with its HTTP status so the console can tell 403 from an outage. */
export class AdminError extends Error {
  constructor(readonly status: number, message: string) {
    super(message);
  }
}

async function get<T>(url: string, signal?: AbortSignal, init?: RequestInit): Promise<T> {
  const response = await fetch(url, { ...init, signal, headers: { Accept: "application/json", ...init?.headers } });
  const body = await response.json().catch(() => null) as { data?: T; error?: { message?: string } } | null;
  if (!response.ok || !body?.data) throw new AdminError(response.status, body?.error?.message ?? `API request failed (${response.status})`);
  return body.data;
}

/** The admin console's API; only allowed Tailscale users get past /admin/me. */
export const adminAPI = {
  me: (signal?: AbortSignal) => get<AdminSession>("/api/v1/admin/me", signal),
  dataStatus: (signal?: AbortSignal) => get<DataStatus>("/api/v1/admin/data-status", signal),
  system: (signal?: AbortSignal) => get<SystemStatus>("/api/v1/admin/system", signal),
  jobs: (signal?: AbortSignal) => get<AdminJob[]>("/api/v1/admin/jobs", signal),
  /** Queues a job; fails with status 409 when one of the kind is already pending. */
  enqueue: (kind: JobKind, params?: SeasonsParams) => get<AdminJob>("/api/v1/admin/jobs", undefined, {
    method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ kind, params }),
  }),
  openings: (signal?: AbortSignal) => get<Openings>("/api/v1/admin/openings", signal),
  /** Sets, or with openedOn null removes, an opening date; returns every date. */
  saveOpening: (edit: OpeningEdit) => get<Openings>("/api/v1/admin/openings", undefined, {
    method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify(edit),
  }),
};
