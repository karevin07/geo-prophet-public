import { lazy, Suspense, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { AlertCircle, Building2, Calculator, Info, ListOrdered, Ruler, RotateCw } from "lucide-react";
import type { BuildingProfile, Estimate, EstimateQuery, RentEstimate, TransactionAPI } from "../../lib/api";
import { formatRent, formatYield } from "../../lib/rent";
import { useLanguage } from "../../lib/i18n";
import { formatWan } from "../../lib/price-scale";
import { ESTIMATE_TYPES, type EstimateState } from "../../lib/url-state";
import { cn, fill } from "../../lib/utils";
import FacilitySummary, { useNearbyFacilities, type FetchNearbyFacilities } from "../FacilitySummary";
import SearchBox from "../SearchBox";

const ComparablesMap = lazy(() => import("./ComparablesMap"));

const SHOWN_COMPARABLES = 6;

interface Props {
  api: TransactionAPI;
  state: EstimateState;
  onChange: (update: Partial<EstimateState>) => void;
  fetchNearbyFacilities?: FetchNearbyFacilities;
}

/** The query for a complete form, or null while the address or area is missing. */
function toQuery(state: EstimateState): EstimateQuery | null {
  if (!state.address || state.lat === null || state.lng === null || !state.area) return null;
  return {
    address: state.address, lat: state.lat, lng: state.lng, type: state.type, area: state.area,
    age: state.age, floor: state.floor, totalFloors: state.totalFloors, parking: state.parking,
  };
}

/** TWD in 萬 without decimals, e.g. 39,100,000 → "3,910". */
function wanTotal(value: number | null): string {
  return value === null ? "—" : formatWan(value, 0);
}

export default function EstimatePage({ api, state, onChange, fetchNearbyFacilities }: Props) {
  const { t } = useLanguage();
  // The estimate runs for the submitted form: on 估價, or at once for a shared link.
  const [submitted, setSubmitted] = useState<EstimateQuery | null>(() => toQuery(state));
  const [estimate, setEstimate] = useState<Estimate | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [retry, setRetry] = useState(0);
  const [profile, setProfile] = useState<BuildingProfile | null>(null);
  const [showAll, setShowAll] = useState(false);
  // The building's sales fill in only what is still empty, so a shared link keeps its values.
  const stateRef = useRef(state);
  stateRef.current = state;
  const query = toQuery(state);

  useEffect(() => {
    if (!state.address || state.lat === null || state.lng === null) {
      setProfile(null);
      return;
    }
    const controller = new AbortController();
    api.estimateBuilding(state.address, state.lat, state.lng, controller.signal).then((value) => {
      if (controller.signal.aborted) return;
      setProfile(value);
      if (value.sales === 0) return;
      const current = stateRef.current;
      const update: Partial<EstimateState> = {};
      // A form without an age is a fresh address, so its type is only a default.
      if (current.age === null && (ESTIMATE_TYPES as readonly string[]).includes(value.category)) update.type = value.category as EstimateState["type"];
      if (current.age === null && value.completedYear) update.age = Math.max(0, new Date().getFullYear() - value.completedYear);
      if (!current.totalFloors && value.totalFloors) update.totalFloors = value.totalFloors;
      if (Object.keys(update).length > 0) onChange(update);
    }).catch(() => {
      if (!controller.signal.aborted) setProfile(null);
    });
    return () => controller.abort();
  }, [api, state.address, state.lat, state.lng, onChange]);

  useEffect(() => {
    if (!submitted) return;
    const controller = new AbortController();
    setLoading(true);
    setError(null);
    setShowAll(false);
    api.estimate(submitted, controller.signal).then((value) => {
      if (!controller.signal.aborted) setEstimate(value);
    }).catch((cause) => {
      if (!controller.signal.aborted) setError(cause instanceof Error ? cause.message : t("estimate.error"));
    }).finally(() => {
      if (!controller.signal.aborted) setLoading(false);
    });
    return () => controller.abort();
  }, [api, submitted, retry]);

  const stale = submitted !== null && JSON.stringify(submitted) !== JSON.stringify(query);
  const control = "mt-1.5 block h-10 w-full rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-800 tabular-nums";
  const numberInput = (value: number | null, set: (value: number | null) => void, unit: string, label: string, hint?: string) => (
    <label className="block text-xs font-medium text-slate-600">
      {label}
      <span className="relative block">
        <input type="number" inputMode="decimal" min={0} value={value ?? ""} aria-label={label}
          onChange={(event) => set(event.target.value === "" ? null : Number(event.target.value))} className={cn(control, "pr-10")} />
        <span className="pointer-events-none absolute right-3 top-1/2 translate-y-[-30%] text-xs text-slate-400">{unit}</span>
      </span>
      {hint && <span className="mt-1 block text-[11px] font-normal text-slate-400">{hint}</span>}
    </label>
  );

  return <main className="h-full overflow-y-auto bg-canvas">
    <div className="mx-auto max-w-[1360px] space-y-5 px-4 py-5 sm:px-6 sm:py-7">
      <header>
        <p className="text-xs font-semibold tracking-wide text-brand-700">GEOPROPHET · {t("nav.estimate")}</p>
        <h1 className="mt-1 text-2xl font-semibold text-slate-900">{t("estimate.title")}</h1>
        <p className="mt-2 max-w-3xl text-sm text-slate-600">{t("estimate.subtitle")}</p>
      </header>

      <div className="grid items-start gap-5 lg:grid-cols-[380px_minmax(0,1fr)]">
        <form className="space-y-4 rounded-2xl border border-slate-200 bg-white p-5 shadow-[0_4px_16px_rgba(15,23,42,0.04)] lg:sticky lg:top-4"
          onSubmit={(event) => {
            event.preventDefault();
            if (query) setSubmitted(query);
          }}>
          <h2 className="flex items-center gap-2 text-base font-semibold text-slate-900"><Building2 className="h-4 w-4 text-brand-600" />{t("estimate.form")}</h2>
          <div>
            <p className="mb-1.5 text-xs font-medium text-slate-600">{t("estimate.address")}</p>
            <SearchBox api={api} kinds={["address"]} initialText={state.address} placeholder={t("estimate.addressPlaceholder")}
              onSelect={(place) => onChange({ address: place.label, lat: place.lat, lng: place.lng, age: null, totalFloors: 0 })} />
            {profile && profile.address === building(state.address) && (
              <p className="mt-2 flex gap-2 rounded-lg border border-brand-100 bg-brand-50/60 px-3 py-2 text-xs text-brand-700">
                <Info className="mt-0.5 h-3.5 w-3.5 shrink-0" />
                {profile.sales > 0 ? fill(t("estimate.profile"), {
                  count: profile.sales,
                  details: [profile.category, profile.completedYear && fill(t("estimate.profile.year"), { year: profile.completedYear }),
                    profile.totalFloors && fill(t("estimate.profile.floors"), { floors: profile.totalFloors })].filter(Boolean).join("、"),
                }) : t("estimate.profile.none")}
              </p>
            )}
          </div>
          <div>
            <p className="mb-1.5 text-xs font-medium text-slate-600">{t("estimate.type")}</p>
            <div className="grid grid-cols-2 gap-2" role="group" aria-label={t("estimate.type")}>
              {ESTIMATE_TYPES.map((type) => <button key={type} type="button" onClick={() => onChange({ type })} aria-pressed={state.type === type}
                className={cn("rounded-lg border py-2 text-sm transition", state.type === type ? "border-brand-600 bg-brand-50 font-medium text-brand-700" : "border-slate-200 text-slate-600 hover:border-brand-300")}>
                {t(`type.${type}`)}
              </button>)}
            </div>
          </div>
          {numberInput(state.area, (area) => onChange({ area }), t("investment.ping"), t("estimate.area"))}
          {numberInput(state.age, (age) => onChange({ age }), t("investment.years"), t("estimate.age"), t("estimate.ageHint"))}
          <div className="grid grid-cols-2 gap-3">
            {numberInput(state.floor || null, (floor) => onChange({ floor: floor ?? 0 }), t("estimate.floorUnit"), t("estimate.floor"))}
            {numberInput(state.totalFloors || null, (totalFloors) => onChange({ totalFloors: totalFloors ?? 0 }), t("estimate.floorsUnit"), t("estimate.totalFloors"))}
          </div>
          <label className="flex items-center gap-2 text-sm text-slate-700">
            <input type="checkbox" checked={state.parking} onChange={(event) => onChange({ parking: event.target.checked })} className="h-4 w-4 accent-brand-600" />
            {t("estimate.parking")}
          </label>
          <button type="submit" disabled={!query} className="flex h-11 w-full items-center justify-center gap-2 rounded-lg bg-brand-600 text-sm font-semibold text-white shadow-sm transition hover:bg-brand-700 disabled:cursor-not-allowed disabled:bg-slate-300">
            <Calculator className="h-4 w-4" />{estimate && submitted ? t("estimate.again") : t("estimate.submit")}
          </button>
          {!query && <p className="text-center text-[11px] text-slate-400">{t("estimate.needs")}</p>}
          {stale && estimate && <p className="text-center text-[11px] text-amber-600">{t("estimate.stale")}</p>}
        </form>

        <div className="min-w-0 space-y-4">
          {!submitted && <div className="rounded-2xl border border-dashed border-slate-300 bg-white/60 p-8 text-center text-sm text-slate-500">{t("estimate.empty")}</div>}
          {loading && <p role="status" className="text-sm text-slate-500">{t("status.loading")}</p>}
          {error && <div role="alert" className="flex items-center justify-between gap-3 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
            <span className="flex items-center gap-2"><AlertCircle className="h-4 w-4" />{error}</span>
            <button type="button" onClick={() => setRetry((value) => value + 1)} className="flex items-center gap-1 font-semibold"><RotateCw className="h-4 w-4" />{t("investment.retry")}</button>
          </div>}
          {estimate && submitted && !loading && !error && (estimate.sufficient
            ? <Result estimate={estimate} query={submitted} showAll={showAll} onShowAll={() => setShowAll(true)} fetchNearbyFacilities={fetchNearbyFacilities} />
            : <div className="rounded-2xl border border-amber-200 bg-amber-50 p-5 text-sm text-amber-900">
              {fill(t("estimate.insufficient"), { count: estimate.comparableSales, minimum: estimate.minimumSample })}
            </div>)}
        </div>
      </div>
    </div>
  </main>;
}

/** The door number part of an address, as the API keys buildings. */
function building(address: string): string {
  return address.match(/^(.*?\d+號)/)?.[1] ?? "";
}

function Result({ estimate, query, showAll, onShowAll, fetchNearbyFacilities }: { estimate: Estimate; query: EstimateQuery; showAll: boolean; onShowAll: () => void; fetchNearbyFacilities?: FetchNearbyFacilities }) {
  const { t } = useLanguage();
  const facilities = useNearbyFacilities(fetchNearbyFacilities, query.lat, query.lng);
  const home = useMemo(() => ({ lat: query.lat, lng: query.lng }), [query.lat, query.lng]);
  const low = estimate.lowPrice ?? 0, high = estimate.highPrice ?? 0, total = estimate.totalPrice ?? 0;
  // The bar spans the range with a margin on both sides.
  const scaleLow = low - (high - low) * 0.15, scaleHigh = high + (high - low) * 0.15;
  const position = (value: number) => `${((value - scaleLow) / (scaleHigh - scaleLow)) * 100}%`;
  const confidence = estimate.confidence || "low";
  const reason = confidence === "high"
    ? fill(t("estimate.reason.high"), { count: estimate.sameBuildingSales })
    : confidence === "medium" ? fill(t("estimate.reason.medium"), { count: estimate.comparableSales })
      : t("estimate.reason.low");
  const percent = (value: number) => `${value > 0 ? "+" : ""}${Math.round(value * 100)}%`;
  const shown = showAll ? estimate.comparables : estimate.comparables.slice(0, SHOWN_COMPARABLES);

  return <>
    <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-[0_4px_16px_rgba(15,23,42,0.04)]">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <p className="flex items-center gap-2 text-xs font-medium text-slate-500">{t("estimate.total")}
            <span className={cn("rounded-full px-2 py-0.5 text-[11px] font-semibold",
              confidence === "high" ? "bg-brand-50 text-brand-700" : confidence === "medium" ? "bg-sky-50 text-sky-700" : "bg-amber-50 text-amber-700")}>
              {t(`estimate.confidence.${confidence}`)}
            </span>
          </p>
          <p className="mt-2 flex items-baseline gap-2"><span className="text-5xl font-semibold tabular-nums text-slate-900">{wanTotal(estimate.totalPrice)}</span><span className="text-lg text-slate-500">{t("rents.unit.wan")}</span></p>
          <p className="mt-2 text-sm text-slate-600">
            <span className="font-semibold text-brand-700">{fill(t("estimate.perPing"), { price: formatWan(estimate.pricePerPing ?? 0) })}</span>
            {" · "}{fill(t("estimate.areaNote"), { area: query.area })}
            {estimate.parkingPrice !== null && <>{" · "}{fill(t("estimate.parkingNote"), { price: wanTotal(estimate.parkingPrice) })}</>}
          </p>
        </div>
        <div className="max-w-sm rounded-xl border border-slate-100 bg-slate-50/70 p-3 text-xs text-slate-600">
          <p className="font-semibold text-slate-800">{t("estimate.reasonTitle")}</p>
          <p className="mt-1">{reason}</p>
        </div>
      </div>

      <div className="mt-6">
        <div className="flex flex-wrap items-baseline justify-between gap-2 text-xs text-slate-500">
          <span>{t("estimate.range")}</span>
          <span className="tabular-nums text-slate-700">{wanTotal(low)} – {wanTotal(high)} {t("rents.unit.wan")}
            <span className="text-slate-400">（{fill(t("estimate.rangePerPing"), { low: formatWan(estimate.lowPricePerPing ?? 0), high: formatWan(estimate.highPricePerPing ?? 0) })}）</span></span>
        </div>
        <div className="relative mt-3 h-3 rounded-full bg-slate-100">
          <div className="absolute inset-y-0 rounded-full bg-brand-100" style={{ left: position(low), right: `calc(100% - ${position(high)})` }} />
          <div className="absolute top-1/2 h-5 w-5 -translate-x-1/2 -translate-y-1/2 rounded-full border-4 border-white bg-brand-600 shadow" style={{ left: position(total) }} />
        </div>
        <p className="mt-3 text-[11px] text-slate-400">{fill(t("estimate.backtest"), {
          confidence: t(`estimate.level.${confidence}`), low: percent(estimate.rangeLow - 1), high: percent(estimate.rangeHigh - 1),
          error: (estimate.backtestMedianError * 100).toFixed(1),
        })}</p>
      </div>
    </section>

    {estimate.rent && <RentCard rent={estimate.rent} area={query.area} />}

    <div className="grid gap-3 sm:grid-cols-3">
      <Stat icon={<ListOrdered className="h-4 w-4" />} label={t("estimate.stat.count")} value={estimate.comparableSales.toLocaleString()} unit={t("estimate.unit.sales")} />
      <Stat icon={<Building2 className="h-4 w-4" />} label={t("estimate.stat.same")} value={String(estimate.sameBuildingSales)} unit={t("estimate.unit.sales")} accent />
      <Stat icon={<Ruler className="h-4 w-4" />} label={t("estimate.stat.distance")} value={String(estimate.maxDistanceM)} unit="m" />
    </div>

    {fetchNearbyFacilities && (
      <section className="rounded-2xl border border-slate-200 bg-white p-5 shadow-[0_4px_16px_rgba(15,23,42,0.04)]">
        <FacilitySummary nearby={facilities.nearby} failed={facilities.failed} />
        <p className="mt-2 text-[11px] text-slate-400">{t("facilities.attribution")}</p>
      </section>
    )}

    <section className="rounded-2xl border border-slate-200 bg-white shadow-[0_4px_16px_rgba(15,23,42,0.04)]">
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-slate-100 px-5 py-4">
        <div>
          <h2 className="text-base font-semibold text-slate-900">{t("estimate.comparables")}</h2>
          <p className="mt-1 text-xs text-slate-500">{fill(t("estimate.comparablesHint"), { asOf: estimate.asOf })}</p>
        </div>
        <div className="flex items-center gap-3 text-xs text-slate-600">
          <span className="flex items-center gap-1"><span className="h-2.5 w-2.5 rounded-full bg-[#1f6f78]" />{t("estimate.legend.home")}</span>
          <span className="flex items-center gap-1"><span className="h-2.5 w-2.5 rounded-full bg-[#e8704f]" />{t("estimate.legend.same")}</span>
          <span className="flex items-center gap-1"><span className="h-2.5 w-2.5 rounded-full bg-[#2f4b7c]" />{t("estimate.legend.nearby")}</span>
        </div>
      </div>
      <div className="h-72 border-b border-slate-100">
        <Suspense fallback={<div className="h-full animate-pulse bg-slate-100" />}>
          <ComparablesMap home={home} comparables={estimate.comparables} />
        </Suspense>
      </div>
      <div className="overflow-x-auto"><table className="w-full min-w-[860px] text-left text-sm">
        <thead className="bg-slate-50 text-xs text-slate-500"><tr>
          {(["address", "date", "area", "age", "floor", "price", "adjusted", "distance", "weight"] as const).map((key) =>
            <th key={key} className={cn("whitespace-nowrap px-4 py-3 font-medium", !["address", "weight"].includes(key) && "text-right")}>{t(`estimate.col.${key}`)}</th>)}
        </tr></thead>
        <tbody className="divide-y divide-slate-100">{shown.map((item) => <tr key={item.id} className={cn("[&>td]:whitespace-nowrap", item.sameBuilding && "bg-orange-50/40")}>
          <td className="px-4 py-3 text-slate-800">
            {item.sameBuilding && <span className="mr-2 rounded bg-[#e8704f] px-1.5 py-0.5 text-[10px] font-semibold text-white">{t("estimate.same")}</span>}
            {item.address.replace(/^(台北市|新北市|桃園市)/, "")}
          </td>
          <td className="px-4 py-3 text-right tabular-nums">{item.transactedAt.slice(0, 7)}</td>
          <td className="px-4 py-3 text-right tabular-nums">{item.areaPing.toFixed(1)}</td>
          <td className="px-4 py-3 text-right tabular-nums">{item.age ?? "—"}</td>
          <td className="px-4 py-3 text-right">{item.floorLabel.split(",")[0] || "—"}</td>
          <td className="px-4 py-3 text-right tabular-nums text-slate-400">{formatWan(item.pricePerPing)}</td>
          <td className="px-4 py-3 text-right font-semibold tabular-nums text-brand-700">{formatWan(item.adjustedPricePerPing)}</td>
          <td className="px-4 py-3 text-right tabular-nums">{item.distanceM} m</td>
          <td className="px-4 py-3"><div className="flex items-center gap-2">
            <span className="h-1.5 w-20 overflow-hidden rounded-full bg-slate-100"><span className="block h-full rounded-full" style={{
              width: `${Math.min(100, item.weight / (estimate.comparables[0]?.weight || 1) * 100)}%`, backgroundColor: item.sameBuilding ? "#e8704f" : "#2f4b7c",
            }} /></span>
            <span className="w-10 text-right text-xs tabular-nums text-slate-500">{(item.weight * 100).toFixed(1)}%</span>
          </div></td>
        </tr>)}</tbody>
      </table></div>
      {!showAll && estimate.comparables.length > SHOWN_COMPARABLES && (
        <button type="button" onClick={onShowAll} className="w-full border-t border-slate-100 px-5 py-3 text-sm font-medium text-brand-700 hover:bg-slate-50">
          {fill(t(estimate.comparableSales > estimate.comparables.length ? "estimate.showTop" : "estimate.showAll"), { count: estimate.comparables.length })}
        </button>
      )}
    </section>

    <aside className="rounded-2xl border border-slate-200 bg-white p-5 text-xs leading-6 text-slate-600">
      <h2 className="text-sm font-semibold text-slate-900">{t("estimate.methodTitle")}</h2>
      <ul className="mt-2 list-disc space-y-1 pl-4">
        <li>{t("estimate.method.comparables")}</li>
        <li>{t("estimate.method.adjust")}</li>
        <li>{t("estimate.method.accuracy")}</li>
        <li>{t("estimate.method.limits")}</li>
      </ul>
    </aside>
  </>;
}

function Stat({ icon, label, value, unit, accent }: { icon: ReactNode; label: string; value: string; unit: string; accent?: boolean }) {
  return <div className="flex items-center gap-3 rounded-2xl border border-slate-200 bg-white p-4">
    <span className={cn("rounded-xl p-2.5", accent ? "bg-orange-50 text-[#e8704f]" : "bg-brand-50 text-brand-700")}>{icon}</span>
    <div><p className="text-xs text-slate-500">{label}</p><p className="mt-0.5 flex items-baseline gap-1"><span className="text-2xl font-semibold tabular-nums text-slate-900">{value}</span><span className="text-xs text-slate-500">{unit}</span></p></div>
  </div>;
}

function RentCard({ rent, area }: { rent: RentEstimate; area: number }) {
  const { t } = useLanguage();
  const card = "rounded-2xl border border-slate-200 bg-white p-6 shadow-[0_4px_16px_rgba(15,23,42,0.04)]";
  const title = <h2 className="flex items-center gap-2 text-base font-semibold text-slate-900">{t("estimate.rent.title")}
    {rent.sufficient && rent.confidence && <span className={cn("rounded-full px-2 py-0.5 text-[11px] font-semibold",
      rent.confidence === "high" ? "bg-brand-50 text-brand-700" : rent.confidence === "medium" ? "bg-sky-50 text-sky-700" : "bg-amber-50 text-amber-700")}>
      {t(`estimate.confidence.${rent.confidence}`)}
    </span>}
  </h2>;
  if (!rent.supported) return <section className={card}>{title}<p className="mt-2 text-sm text-slate-500">{t("estimate.rent.unsupported")}</p></section>;
  if (!rent.sufficient || rent.monthlyRent === null) {
    return <section className={card}>{title}<p className="mt-2 text-sm text-slate-500">{fill(t("estimate.rent.insufficient"), { count: rent.leaseCount, minimum: rent.minimumSample })}</p></section>;
  }
  const low = rent.lowMonthlyRent ?? 0, high = rent.highMonthlyRent ?? 0, value = rent.monthlyRent;
  const scaleLow = low - (high - low) * 0.15, scaleHigh = high + (high - low) * 0.15;
  const position = (amount: number) => `${((amount - scaleLow) / (scaleHigh - scaleLow)) * 100}%`;
  const percent = (ratio: number) => `${ratio > 0 ? "+" : ""}${Math.round(ratio * 100)}%`;
  return <section className={card}>
    {title}
    <div className="mt-4 grid gap-5 lg:grid-cols-[minmax(0,1fr)_260px]">
      <div>
        <p className="flex items-baseline gap-2"><span className="text-4xl font-semibold tabular-nums text-slate-900">{formatWan(value)}</span><span className="text-sm text-slate-500">{t("estimate.rent.perMonth")}</span></p>
        <p className="mt-1 text-sm text-slate-600"><span className="font-semibold text-brand-700">{fill(t("estimate.rent.perPing"), { rent: formatRent(rent.rentPerPing) })}</span> · {fill(t("estimate.areaNote"), { area })}</p>
        <div className="mt-4 flex flex-wrap items-baseline justify-between gap-2 text-xs text-slate-500">
          <span>{t("estimate.range")}</span>
          <span className="tabular-nums text-slate-700">{formatWan(low)} – {formatWan(high)} {t("estimate.rent.perMonth")}</span>
        </div>
        <div className="relative mt-2 h-3 rounded-full bg-slate-100">
          <div className="absolute inset-y-0 rounded-full bg-[#4fa3a5]/30" style={{ left: position(low), right: `calc(100% - ${position(high)})` }} />
          <div className="absolute top-1/2 h-5 w-5 -translate-x-1/2 -translate-y-1/2 rounded-full border-4 border-white bg-[#4fa3a5] shadow" style={{ left: position(value) }} />
        </div>
      </div>
      <div className="space-y-2">
        <div className="rounded-xl border border-brand-200 bg-brand-50/60 p-3">
          <p className="flex items-baseline justify-between gap-2"><span className="text-xs text-slate-600">{t("estimate.rent.yield")}</span><span className="text-xl font-semibold tabular-nums text-brand-700">{formatYield(rent.grossYield)}</span></p>
          <p className="mt-0.5 text-[11px] text-slate-500">{t("estimate.rent.yieldNote")}</p>
        </div>
        <div className="rounded-xl border border-slate-100 bg-slate-50/60 p-3">
          <p className="flex items-baseline justify-between gap-2"><span className="text-xs text-slate-600">{t("estimate.rent.districtYield")}</span><span className="text-lg font-semibold tabular-nums text-slate-800">{formatYield(rent.districtGrossYield)}</span></p>
          <p className="mt-0.5 text-[11px] text-slate-500">{t("estimate.rent.districtYieldNote")}</p>
        </div>
      </div>
    </div>

    {rent.leases.length > 0 && <div className="mt-5">
      <h3 className="mb-2 flex items-center justify-between text-xs font-medium text-slate-600">{t("estimate.rent.leases")}
        <span className="font-normal text-slate-400">{fill(t("estimate.rent.leaseCount"), { count: rent.leaseCount, same: rent.sameBuildingLeases })}</span></h3>
      <div className="overflow-x-auto rounded-xl border border-slate-100"><table className="w-full min-w-[560px] text-left text-sm">
        <thead className="bg-slate-50 text-xs text-slate-500"><tr>
          {(["address", "date", "area", "rent", "perPing", "distance"] as const).map((key) =>
            <th key={key} className={cn("whitespace-nowrap px-3 py-2 font-medium", key !== "address" && "text-right")}>{t(`estimate.rent.col.${key}`)}</th>)}
        </tr></thead>
        <tbody className="divide-y divide-slate-100">{rent.leases.slice(0, 5).map((lease) => <tr key={lease.id} className={cn("[&>td]:whitespace-nowrap", lease.sameBuilding && "bg-orange-50/40")}>
          <td className="px-3 py-2 text-slate-800">
            {lease.sameBuilding && <span className="mr-2 rounded bg-[#e8704f] px-1.5 py-0.5 text-[10px] font-semibold text-white">{t("estimate.same")}</span>}
            {lease.address.replace(/^(台北市|新北市|桃園市)/, "")}
          </td>
          <td className="px-3 py-2 text-right tabular-nums">{lease.leasedAt.slice(0, 7)}</td>
          <td className="px-3 py-2 text-right tabular-nums">{lease.areaPing.toFixed(1)}</td>
          <td className="px-3 py-2 text-right tabular-nums">{formatWan(lease.monthlyRent)}</td>
          <td className="px-3 py-2 text-right font-semibold tabular-nums text-brand-700">{formatRent(lease.rentPerPing)}</td>
          <td className="px-3 py-2 text-right tabular-nums">{lease.distanceM} m</td>
        </tr>)}</tbody>
      </table></div>
    </div>}
    <p className="mt-3 text-[11px] text-slate-400">{fill(t("estimate.rent.backtest"), {
      confidence: t(`estimate.level.${rent.confidence || "low"}`), low: percent(rent.rangeLow - 1), high: percent(rent.rangeHigh - 1),
      error: (rent.backtestMedianError * 100).toFixed(1),
    })}</p>
  </section>;
}
