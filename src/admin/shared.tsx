import type { ImportRun } from "../lib/api";
import { useLanguage } from "../lib/i18n";
import { cn } from "../lib/utils";

export const CARD = "rounded-2xl border border-slate-200 bg-white shadow-[0_4px_16px_rgba(15,23,42,0.04)]";

export type Tone = "green" | "red" | "amber" | "grey";

/** 09/30 13:01 in local time; the year only when it is not this year. */
export function shortTime(value: string | null | undefined, withYear = false) {
  if (!value) return "—";
  const date = new Date(value);
  const pad = (number: number) => String(number).padStart(2, "0");
  const day = `${pad(date.getMonth() + 1)}/${pad(date.getDate())}`;
  const year = withYear || date.getFullYear() !== new Date().getFullYear() ? `${date.getFullYear()}/` : "";
  return `${year}${day} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

/** 132781039 → 127 MB, using 1024-based units like df and du. */
export function formatBytes(bytes: number) {
  const units = ["B", "KB", "MB", "GB", "TB"];
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  return `${value >= 100 || unit === 0 ? Math.round(value) : value.toFixed(1)} ${units[unit]}`;
}

export function Dot({ tone }: { tone: Tone }) {
  const color = { green: "bg-emerald-500", red: "bg-red-500", amber: "bg-amber-400", grey: "bg-slate-300" }[tone];
  return <span className={cn("inline-block h-2 w-2 shrink-0 rounded-full", color)} />;
}

export function runTone(run: ImportRun | undefined): Tone {
  if (!run) return "grey";
  return run.status === "succeeded" ? "green" : run.status === "running" ? "amber" : "red";
}

export function StatusPill({ run }: { run: ImportRun }) {
  const { t } = useLanguage();
  const tone = runTone(run);
  const classes = { green: "bg-emerald-50 text-emerald-700", red: "bg-red-50 text-red-700", amber: "bg-amber-50 text-amber-700", grey: "bg-slate-100 text-slate-500" }[tone];
  return (
    <span className={cn("inline-flex items-center gap-1 whitespace-nowrap rounded-full px-2 py-0.5 text-xs font-medium", classes)}>
      <Dot tone={tone} />
      {t(`data.status.${run.status}`)}
      {run.errorKind && <span className="font-normal">（{t(`data.errorKind.${run.errorKind}`)}）</span>}
    </span>
  );
}
