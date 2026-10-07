import { lazy, Suspense } from "react";
import { ChevronLeft, Hexagon, MapPin, SlidersHorizontal } from "lucide-react";
import { cn } from "../lib/utils";
import { useLanguage } from "../lib/i18n";
import { formatWan } from "../lib/price-scale";
import { type BuildingType, type Place, type SummaryWithYoY, type TransactionAPI, type TrendPoint } from "../lib/api";
import SearchBox from "./SearchBox";
import { RangeField, SectionTitle, Switch, TypeChips } from "./ui";
import YoYBadge from "./YoYBadge";
import ExportButton from "./ExportButton";

const TrendChart = lazy(() => import("./TrendChart"));

const WAN = 10_000;

interface FilterPanelProps {
  exportUrl?: string | null;
  api: TransactionAPI;
  onSelectPlace: (place: Place) => void;
  /** Opens the estimate page for an address found by the search. */
  onEstimatePlace: (place: Place) => void;
  summary: SummaryWithYoY | null;
  trends: TrendPoint[];
  isLoading: boolean;
  error: string | null;
  priceRange: [number, number];
  setPriceRange: (value: [number, number]) => void;
  areaRange: [number, number];
  setAreaRange: (value: [number, number]) => void;
  selectedTypes: BuildingType[];
  setSelectedTypes: (value: BuildingType[]) => void;
  includeSpecial: boolean;
  setIncludeSpecial: (value: boolean) => void;
  onReset: () => void;
  h3Resolution: number;
  setH3Resolution: (value: number) => void;
  viewMode: "hex" | "points";
  setViewMode: (value: "hex" | "points") => void;
  isOpen: boolean;
  setIsOpen: (value: boolean) => void;
}

export default function FilterPanel(props: FilterPanelProps) {
  const { t } = useLanguage();
  const { isOpen, setIsOpen, summary } = props;
  const yoy = summary?.yoy;

  if (!isOpen) {
    return (
      <button
        onClick={() => setIsOpen(true)}
        className="absolute left-4 top-4 z-30 flex items-center gap-2 rounded-xl border border-slate-200 bg-white px-4 py-2.5 text-sm font-medium text-slate-700 shadow-[0_4px_16px_rgba(15,23,42,0.08)] hover:text-brand-700"
      >
        <SlidersHorizontal className="h-4 w-4 text-brand-600" />
        {t("panel.expand")}
      </button>
    );
  }

  return (
    <aside className="absolute inset-x-3 top-3 z-30 flex max-h-[calc(100%-7rem)] flex-col overflow-hidden rounded-2xl border border-slate-200/80 bg-white/95 shadow-[0_4px_16px_rgba(15,23,42,0.08)] backdrop-blur sm:inset-x-auto sm:bottom-4 sm:left-4 sm:top-4 sm:max-h-none sm:w-80">

      <div className="flex-1 space-y-6 overflow-y-auto px-5 py-5">
        <div className="flex items-center gap-2">
          <div className="min-w-0 flex-1">
            <SearchBox api={props.api} onSelect={props.onSelectPlace} onEstimate={props.onEstimatePlace} />
          </div>
          <button onClick={() => setIsOpen(false)} aria-label={t("panel.collapse")} title={t("panel.collapse")} className="rounded-md p-2 text-slate-400 hover:bg-slate-100 hover:text-slate-600">
            <ChevronLeft className="h-4 w-4" />
          </button>
        </div>

        <section>
          <SectionTitle>{t("summary.title")}</SectionTitle>
          <div className="grid grid-cols-3 gap-2">
            <Stat label={t("summary.count")} value={summary ? summary.count.toLocaleString() : "—"} unit={t("summary.countUnit")} change={yoy?.count} />
            <Stat label={t("summary.avgPrice")} value={summary?.count ? formatWan(summary.avgPricePerPing) : "—"} unit={t("summary.avgUnit")} change={yoy?.avgPricePerPing} highlight />
            <Stat label={t("summary.median")} value={summary?.count ? formatWan(summary.medianTotalPrice, 0) : "—"} unit={t("summary.medianUnit")} change={yoy?.medianTotalPrice} />
          </div>
          {yoy?.through && (
            <p className="mt-2 text-[11px] text-slate-400">
              {t("yoy.window").replace("{through}", yoy.through.replace("-", "/")).replace("{months}", yoy.basis.replace("m", ""))}
            </p>
          )}
        </section>

        <section className="space-y-3">
          <SectionTitle aside={props.viewMode === "hex" ? `H3 Res-${props.h3Resolution}` : undefined}>{t("view.title")}</SectionTitle>
          <div className="grid grid-cols-2 rounded-lg bg-slate-100 p-1">
            {([["hex", Hexagon, "view.hex"], ["points", MapPin, "view.points"]] as const).map(([mode, Icon, label]) => (
              <button
                key={mode}
                onClick={() => props.setViewMode(mode)}
                className={cn(
                  "flex items-center justify-center gap-1.5 rounded-md py-1.5 text-sm font-medium transition",
                  props.viewMode === mode ? "bg-brand-600 text-white shadow-sm" : "text-slate-600 hover:text-slate-900",
                )}
              >
                <Icon className="h-4 w-4" />
                {t(label)}
              </button>
            ))}
          </div>
          {props.viewMode === "hex" && (
            <div>
              <div className="mb-1 text-xs text-slate-500">{t("view.grid")}</div>
              <input
                type="range" min={7} max={10} step={1} value={props.h3Resolution}
                onChange={(event) => props.setH3Resolution(Number(event.target.value))}
                className="w-full"
                aria-label={t("view.grid")}
              />
              <div className="flex justify-between text-[11px] text-slate-400">
                <span>{t("view.city")}</span>
                <span>{t("view.district")}</span>
                <span>{t("view.street")}</span>
              </div>
            </div>
          )}
        </section>

        <section className="space-y-4">
          <SectionTitle action={<button onClick={props.onReset} className="text-xs font-medium text-brand-600 hover:text-brand-700">{t("filter.reset")}</button>}>
            {t("filter.title")}
          </SectionTitle>
          <RangeField
            label={t("filter.price")}
            unit="萬"
            value={[props.priceRange[0] / WAN, props.priceRange[1] / WAN]}
            onChange={([low, high]) => props.setPriceRange([low * WAN, high * WAN])}
          />
          <RangeField label={t("filter.area")} unit="坪" value={props.areaRange} onChange={props.setAreaRange} />
          <TypeChips label={t("filter.type")} selected={props.selectedTypes} onChange={props.setSelectedTypes} />
          <label className="flex cursor-pointer items-start justify-between gap-3">
            <span>
              <span className="block text-xs font-medium text-slate-600">{t("filter.special")}</span>
              <span className="block text-[11px] text-slate-400">{t("filter.specialHint")}</span>
            </span>
            <Switch checked={props.includeSpecial} onChange={props.setIncludeSpecial} />
          </label>
        </section>

        <section>
          <div className="mb-3 flex items-center justify-between gap-2">
            <h2 className="text-xs font-semibold tracking-wide text-slate-500">{t("chart.title")}</h2>
            {!summary?.yoy?.basis.startsWith("3") && summary?.count ? (
              <span className="flex items-center gap-1.5 text-xs font-semibold text-brand-700">
                {formatWan(summary.avgPricePerPing)} {t("chart.unit")}
                <YoYBadge value={yoy?.avgPricePerPing} suffix={t("yoy.label")} />
              </span>
            ) : null}
          </div>
          <Suspense fallback={<div className="h-36 animate-pulse rounded-lg bg-slate-100" />}>
            <TrendChart data={props.trends} />
          </Suspense>
          {props.trends.some((point) => point.provisional) && (
            <p className="mt-1 text-[11px] leading-snug text-slate-400">{t("chart.provisional")}</p>
          )}
        </section>
      </div>

      <div className="border-t border-slate-100 px-5 pt-3"><ExportButton url={props.exportUrl ?? null} /></div>
      <footer className="flex items-center gap-2 px-5 py-3 text-[11px] text-slate-400">
        <span className={cn("h-1.5 w-1.5 shrink-0 rounded-full", props.error ? "bg-rose-500" : props.isLoading ? "animate-pulse bg-amber-400" : "bg-emerald-500")} />
        <span className={cn("truncate", props.error && "text-rose-600")}>{props.error ?? (props.isLoading ? t("status.loading") : t("source"))}</span>
      </footer>
    </aside>
  );
}

function Stat({ label, value, unit, change, highlight }: { label: string; value: string; unit: string; change?: number | null; highlight?: boolean }) {
  return (
    <div className={cn("flex flex-col items-center rounded-xl border px-1.5 py-2.5 text-center", highlight ? "border-brand-100 bg-brand-50" : "border-slate-200")}>
      <div className="text-[11px] text-slate-500">{label}</div>
      <div className={cn("mt-1 text-base font-semibold tabular-nums", highlight ? "text-brand-700" : "text-slate-900")}>{value}</div>
      <div className="text-[10px] text-slate-400">{unit}</div>
      {change != null && <YoYBadge value={change} className="mt-1.5" />}
    </div>
  );
}
