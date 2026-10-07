import { useEffect, useState, type ReactNode } from "react";
import { cn } from "../lib/utils";
import { useLanguage } from "../lib/i18n";
import { BUILDING_TYPES, type BuildingType } from "../lib/api";

export function SectionTitle({ children, aside, action }: { children: ReactNode; aside?: string; action?: ReactNode }) {
  return (
    <div className="mb-3 flex items-center justify-between">
      <h2 className="text-xs font-semibold tracking-wide text-slate-500">{children}</h2>
      {aside && <span className="text-[11px] text-slate-400">{aside}</span>}
      {action}
    </div>
  );
}

/** Two number inputs that commit on blur or Enter, so typing does not refetch on every key. */
export function RangeField({ label, unit, value, onChange }: { label: string; unit: string; value: [number, number]; onChange: (value: [number, number]) => void }) {
  const [draft, setDraft] = useState(value.map(String) as [string, string]);
  useEffect(() => setDraft(value.map(String) as [string, string]), [value[0], value[1]]);
  const commit = () => {
    const low = Math.max(0, Number(draft[0]) || 0);
    const high = Math.max(low, Number(draft[1]) || 0);
    onChange([low, high]);
  };
  const input = (index: 0 | 1) => (
    <div className="relative flex-1">
      <input
        type="number" min={0} inputMode="numeric"
        value={draft[index]}
        onChange={(event) => setDraft((current) => index === 0 ? [event.target.value, current[1]] : [current[0], event.target.value])}
        onBlur={commit}
        onKeyDown={(event) => event.key === "Enter" && commit()}
        aria-label={`${label} ${index === 0 ? "min" : "max"}`}
        className="h-9 w-full rounded-lg border border-slate-200 bg-white pl-3 pr-8 text-sm tabular-nums text-slate-800 outline-none focus:border-brand-500 focus:ring-2 focus:ring-brand-100"
      />
      <span className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-xs text-slate-400">{unit}</span>
    </div>
  );
  return (
    <div>
      <div className="mb-2 text-xs font-medium text-slate-600">{label}</div>
      <div className="flex items-center gap-2">
        {input(0)}
        <span className="text-slate-300">~</span>
        {input(1)}
      </div>
    </div>
  );
}

export function Switch({ checked, onChange }: { checked: boolean; onChange: (value: boolean) => void }) {
  return (
    <button
      role="switch"
      aria-checked={checked}
      onClick={(event) => {
        event.preventDefault();
        onChange(!checked);
      }}
      className={cn("relative mt-0.5 h-5 w-9 shrink-0 rounded-full transition", checked ? "bg-brand-600" : "bg-slate-200")}
    >
      <span className={cn("absolute top-0.5 h-4 w-4 rounded-full bg-white shadow transition", checked ? "left-[18px]" : "left-0.5")} />
    </button>
  );
}

export function TypeChips({ label, selected, onChange }: { label: string; selected: BuildingType[]; onChange: (value: BuildingType[]) => void }) {
  const { t } = useLanguage();
  return (
    <div>
      <div className="mb-2 text-xs font-medium text-slate-600">{label}</div>
      <div className="flex flex-wrap gap-2">
        {BUILDING_TYPES.map((type) => {
          const isSelected = selected.includes(type);
          return (
            <button
              key={type}
              aria-pressed={isSelected}
              onClick={() => onChange(isSelected ? selected.filter((item) => item !== type) : [...selected, type])}
              className={cn(
                "rounded-full border px-3 py-1 text-xs font-medium transition",
                isSelected ? "border-brand-100 bg-brand-50 text-brand-700" : "border-slate-200 bg-white text-slate-500 hover:border-slate-300",
              )}
            >
              {t(`type.${type}`)}
            </button>
          );
        })}
      </div>
    </div>
  );
}
