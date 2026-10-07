import { cn } from "../lib/utils";

/** A small ▲/▼ pill for a relative change such as 0.041 (+4.1%), red for rising and green
 * for falling as is usual in Taiwan. */
export default function YoYBadge({ value, suffix, className }: { value: number | null | undefined; suffix?: string; className?: string }) {
  if (value == null || !Number.isFinite(value)) return null;
  const rounded = Math.round(value * 1000) / 10;
  const up = rounded > 0;
  const flat = rounded === 0;
  return (
    <span
      className={cn(
        "inline-flex items-center gap-0.5 rounded px-1.5 py-0.5 text-[10px] font-semibold tabular-nums",
        flat ? "bg-slate-100 text-slate-500" : up ? "bg-red-100 text-red-700" : "bg-green-100 text-green-700",
        className,
      )}
    >
      {flat ? "–" : up ? "▲" : "▼"} {Math.abs(rounded).toFixed(1)}%{suffix ? ` ${suffix}` : ""}
    </span>
  );
}
