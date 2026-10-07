import { lazy, Suspense, useCallback, useEffect, useMemo, useState } from "react";
import { AlertCircle, ArrowDownUp, CalendarDays, Info, RotateCw } from "lucide-react";
import type { DistrictBoundaries, RentalDistrict, RentalQuery, RentalReport, RentTrend, TransactionAPI } from "../../lib/api";
import { useLanguage } from "../../lib/i18n";
import { formatWan, priceClass, priceColor, PRICE_CLASSES } from "../../lib/price-scale";
import { formatRent, formatYield, rentRows, yieldBreaks } from "../../lib/rent";
import { RENT_AREA_BANDS, RENT_TYPES, type RentsState } from "../../lib/url-state";
import { cn, fill } from "../../lib/utils";
import RentTrendChart, { CITY_COLOR, DISTRICT_COLOR } from "./RentTrendChart";

const ChoroplethMap = lazy(() => import("../ChoroplethMap"));

type Sort = "rentals" | "rent" | "yield";
const CITY_CODES = ["a", "f", "h"] as const;
const LEGEND_LABELS = ["rents.legend.low", "rents.legend.midLow", "rents.legend.mid", "rents.legend.high"];

interface Props {
  api: TransactionAPI;
  state: RentsState;
  onChange: (update: Partial<RentsState>) => void;
}

function sortDistricts(districts: RentalDistrict[], sort: Sort): RentalDistrict[] {
  if (sort === "rentals") return districts;
  const metric = (district: RentalDistrict) => sort === "rent" ? district.medianRentPerPing : district.grossYield;
  return [...districts].sort((a, b) => {
    const left = metric(a), right = metric(b);
    if (left === null) return right === null ? b.rentals - a.rentals : 1;
    if (right === null) return -1;
    return right - left;
  });
}

function wan(value: number | null): string {
  return value === null ? "—" : formatWan(value);
}

export default function RentsPage({ api, state, onChange }: Props) {
  const { t } = useLanguage();
  const [report, setReport] = useState<RentalReport | null>(null);
  const [trend, setTrend] = useState<RentTrend | null>(null);
  const [boundaries, setBoundaries] = useState<DistrictBoundaries | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [retry, setRetry] = useState(0);
  const [sort, setSort] = useState<Sort>("rentals");
  const query = useMemo<RentalQuery>(() => {
    const [minArea, maxArea] = RENT_AREA_BANDS[state.areaBand];
    return { city: state.city, type: state.type, minArea, maxArea, includeSocial: state.includeSocial };
  }, [state.city, state.type, state.areaBand, state.includeSocial]);

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError(null);
    setReport(null);
    api.rentalDistricts(query, controller.signal).then((value) => {
      if (!controller.signal.aborted) setReport(value);
    }).catch((cause) => {
      if (!controller.signal.aborted) setError(cause instanceof Error ? cause.message : t("rents.error"));
    }).finally(() => {
      if (!controller.signal.aborted) setLoading(false);
    });
    return () => controller.abort();
  }, [api, query, retry]);

  // The trend is secondary: a failure leaves its card empty.
  useEffect(() => {
    const controller = new AbortController();
    setTrend(null);
    api.rentalTrend(query, state.district, controller.signal).then((value) => {
      if (!controller.signal.aborted) setTrend(value);
    }).catch(() => undefined);
    return () => controller.abort();
  }, [api, query, state.district, retry]);

  useEffect(() => {
    const controller = new AbortController();
    setBoundaries(null);
    api.districtBoundaries(state.city, controller.signal).then(setBoundaries).catch(() => undefined);
    return () => controller.abort();
  }, [api, state.city]);

  const districts = useMemo(() => sortDistricts(report?.districts ?? [], sort), [report, sort]);
  const byDistrict = useMemo(() => new Map((report?.districts ?? []).map((district) => [district.district, district])), [report]);
  const breaks = useMemo(() => yieldBreaks((report?.districts ?? []).map((district) => district.grossYield)), [report]);
  const maxYield = useMemo(() => Math.max(0, ...(report?.districts ?? []).map((district) => district.grossYield ?? 0)), [report]);
  const rows = useMemo(() => rentRows(trend?.city ?? [], trend?.districtPoints ?? []), [trend]);
  const select = useCallback((district: string) => onChange({ district: state.district === district ? "" : district }), [onChange, state.district]);
  const fillOf = useCallback((district: string) => {
    const value = byDistrict.get(district)?.grossYield;
    return value == null ? null : priceColor(value, breaks, 160);
  }, [byDistrict, breaks]);
  const fillKey = useMemo(() => `${breaks?.join()}|${(report?.districts ?? []).map((district) => `${district.district}:${district.grossYield}`).join()}`, [breaks, report]);

  const city = report?.city;
  const control = "mt-2 block h-10 w-full rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-800";

  return <main className="h-full overflow-y-auto bg-canvas">
    <div className="mx-auto max-w-[1360px] space-y-5 px-4 py-5 sm:px-6 sm:py-7">
      <header>
        <p className="text-xs font-semibold tracking-wide text-brand-700">GEOPROPHET · {t("nav.rents")}</p>
        <h1 className="mt-1 text-2xl font-semibold text-slate-900">{t("rents.title")}</h1>
        <p className="mt-2 max-w-3xl text-sm text-slate-600">{t("rents.subtitle")}</p>
      </header>

      <div className="flex gap-3 rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900">
        <Info className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
        <p>{t("rents.caution")}</p>
      </div>

      <section className="flex flex-wrap items-end gap-x-6 gap-y-4 rounded-2xl border border-slate-200 bg-white p-5 shadow-[0_4px_16px_rgba(15,23,42,0.04)]" aria-label={t("rents.filters")}>
        <label className="w-36 text-xs font-medium text-slate-600">{t("rents.city")}
          <select value={state.city} onChange={(event) => onChange({ city: event.target.value as RentsState["city"], district: "" })} className={control}>
            {CITY_CODES.map((code) => <option key={code} value={code}>{t(`investment.city.${code}`)}</option>)}
          </select>
        </label>
        <div>
          <p className="mb-2 text-xs font-medium text-slate-600">{t("rents.type")}</p>
          <div className="flex flex-wrap gap-2" role="group" aria-label={t("rents.type")}>
            {RENT_TYPES.map((type) => <button key={type || "all"} type="button" onClick={() => onChange({ type })} aria-pressed={state.type === type}
              className={cn("rounded-full border px-3 py-1.5 text-sm transition", state.type === type ? "border-brand-600 bg-brand-50 font-medium text-brand-700" : "border-slate-200 text-slate-600 hover:border-brand-300")}>
              {type ? t(`type.${type}`) : t("rents.type.all")}
            </button>)}
          </div>
        </div>
        <label className="w-36 text-xs font-medium text-slate-600">{t("rents.area")}
          <select value={state.areaBand} onChange={(event) => onChange({ areaBand: Number(event.target.value) })} className={control}>
            {RENT_AREA_BANDS.map(([min, max], index) => <option key={index} value={index}>{max === 0 ? t("rents.area.all") : `${min}–${max} ${t("investment.ping")}`}</option>)}
          </select>
        </label>
        <div className="ml-auto flex items-center gap-3 text-sm text-slate-700">
          <button type="button" role="switch" aria-checked={state.includeSocial} aria-label={t("rents.social")} onClick={() => onChange({ includeSocial: !state.includeSocial })}
            className={cn("relative h-6 w-11 shrink-0 rounded-full transition", state.includeSocial ? "bg-brand-600" : "bg-slate-300")}>
            <span className={cn("absolute top-0.5 h-5 w-5 rounded-full bg-white shadow transition", state.includeSocial ? "left-[22px]" : "left-0.5")} />
          </button>
          <span>{t("rents.social")}<span className="block text-xs text-slate-400">{t("rents.socialHint")}</span></span>
        </div>
      </section>

      {loading && <p role="status" className="text-sm text-slate-500">{t("status.loading")}</p>}
      {error && <div role="alert" className="flex items-center justify-between gap-3 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
        <span className="flex items-center gap-2"><AlertCircle className="h-4 w-4" />{error}</span>
        <button type="button" onClick={() => setRetry((value) => value + 1)} className="flex items-center gap-1 font-semibold"><RotateCw className="h-4 w-4" />{t("investment.retry")}</button>
      </div>}

      {report && !report.through && <div className="rounded-xl border border-slate-200 bg-white p-5 text-sm text-slate-600">{t("rents.noCoverage")}</div>}
      {report?.through && city && <>
        <p className="flex items-center gap-2 text-xs text-slate-500"><CalendarDays className="h-3.5 w-3.5" />{fill(t("rents.period"), { from: report.from, through: report.through })}</p>
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <Metric title={t("rents.kpi.rent")} value={formatRent(city.medianRentPerPing)} unit={t("rents.unit.perPing")} note={fill(t("rents.kpi.leases"), { count: city.rentals.toLocaleString() })} />
          <Metric title={t("rents.kpi.monthly")} value={wan(city.medianMonthlyRent)} unit={t("rents.unit.wan")} note={t("rents.kpi.monthlyNote")} />
          <Metric title={t("rents.kpi.price")} value={wan(city.medianPricePerPing)} unit={t("chart.unit")} note={fill(t("rents.kpi.sales"), { count: city.sales.toLocaleString() })} />
          <Metric title={t("rents.kpi.yield")} value={formatYield(city.grossYield)} note={t("rents.kpi.yieldNote")} highlight />
        </div>

        <div className="grid gap-4 lg:grid-cols-5">
          <section className="rounded-2xl border border-slate-200 bg-white p-5 shadow-[0_4px_16px_rgba(15,23,42,0.04)] lg:col-span-3">
            <h2 className="text-base font-semibold text-slate-900">{t("rents.map")}</h2>
            <p className="mb-3 mt-1 text-xs text-slate-500">{t("rents.mapHint")}</p>
            <Suspense fallback={<div className="h-80 animate-pulse rounded-xl bg-slate-100 sm:h-96" />}>
              <ChoroplethMap
                boundaries={boundaries}
                fill={fillOf}
                fillKey={fillKey}
                selected={state.district ? [{ district: state.district, color: DISTRICT_COLOR }] : []}
                onSelect={select}
                hoverContent={(name) => {
                  const district = byDistrict.get(name);
                  if (!district) return <p className="text-slate-400">{t("rents.noLeases")}</p>;
                  return <dl className="space-y-1">
                    <HoverRow label={t("rents.col.yield")} value={formatYield(district.grossYield)} />
                    <HoverRow label={t("rents.col.rent")} value={`${formatRent(district.medianRentPerPing)} ${t("rents.unit.perPing")}`} />
                    <HoverRow label={t("rents.col.price")} value={`${wan(district.medianPricePerPing)} ${t("chart.unit")}`} />
                    <HoverRow label={t("rents.col.rentals")} value={district.rentals.toLocaleString()} />
                  </dl>;
                }}
                legend={<>
                  {[3, 2, 1, 0].map((index) => <span key={index} className="flex items-center gap-1">
                    <span className="h-2.5 w-2.5 rounded-full" style={{ backgroundColor: PRICE_CLASSES[index].hex }} />
                    {t(LEGEND_LABELS[index])}
                    {breaks && <span className="tabular-nums text-slate-400">{
                      index === 3 ? `≥${formatYield(breaks[2], 1)}` : index === 0 ? `<${formatYield(breaks[0], 1)}` : `${formatYield(breaks[index - 1], 1)}–${formatYield(breaks[index], 1)}`
                    }</span>}
                  </span>)}
                  <span className="flex items-center gap-1"><span className="h-2.5 w-2.5 rounded-full bg-slate-300" />{t("rents.lowSample")}</span>
                </>}
              />
            </Suspense>
          </section>

          <section className="flex flex-col rounded-2xl border border-slate-200 bg-white p-5 shadow-[0_4px_16px_rgba(15,23,42,0.04)] lg:col-span-2">
            <div className="flex flex-wrap items-start justify-between gap-2">
              <div>
                <h2 className="text-base font-semibold text-slate-900">{t("rents.trend")}</h2>
                <p className="mt-1 text-xs text-slate-500">{t("rents.trendHint")}</p>
              </div>
              <div className="flex items-center gap-3 text-xs text-slate-600">
                <span className="flex items-center gap-1"><span className="h-2 w-2 rounded-full" style={{ backgroundColor: CITY_COLOR }} />{t(`investment.city.${state.city}`)}</span>
                {state.district && <span className="flex items-center gap-1"><span className="h-2 w-2 rounded-full" style={{ backgroundColor: DISTRICT_COLOR }} />{state.district}</span>}
              </div>
            </div>
            {trend ? <RentTrendChart rows={rows} cityLabel={t(`investment.city.${state.city}`)} districtLabel={state.district || undefined} className="mt-3 h-72 shrink-0 lg:h-auto lg:min-h-72 lg:flex-1" />
              : <div className="mt-3 h-72 shrink-0 animate-pulse rounded-xl bg-slate-100 lg:h-auto lg:min-h-72 lg:flex-1" />}
            <p className="mt-3 text-[11px] text-slate-400">{t("rents.trendNote")}</p>
            {!state.includeSocial && <p className="mt-1 text-[11px] text-slate-400">{t("rents.trendSocial")}</p>}
            {!state.district && <p className="mt-1 text-[11px] text-slate-400">{t("rents.pickDistrict")}</p>}
          </section>
        </div>

        <section className="rounded-2xl border border-slate-200 bg-white shadow-[0_4px_16px_rgba(15,23,42,0.04)]">
          <div className="flex flex-wrap items-center justify-between gap-3 border-b border-slate-100 px-5 py-4">
            <div><h2 className="text-base font-semibold text-slate-900">{t("rents.table")}</h2><p className="mt-1 text-xs text-slate-500">{fill(t("rents.tableHint"), { minimum: report.minimumSample })}</p></div>
            <label className="flex items-center gap-2 text-xs text-slate-600"><ArrowDownUp className="h-4 w-4" />{t("investment.sort")}
              <select value={sort} onChange={(event) => setSort(event.target.value as Sort)} className="h-9 rounded-lg border border-slate-200 bg-white px-2 text-sm text-slate-800">
                <option value="rentals">{t("rents.sort.rentals")}</option><option value="rent">{t("rents.sort.rent")}</option><option value="yield">{t("rents.sort.yield")}</option>
              </select>
            </label>
          </div>
          {districts.length === 0 ? <p className="p-5 text-sm text-slate-500">{t("rents.empty")}</p> : <>
            <div className="hidden overflow-x-auto md:block"><table className="w-full min-w-[900px] text-left text-sm">
              <thead className="bg-slate-50 text-xs text-slate-500"><tr>
                {(["district", "rentals", "rent", "monthly", "sales", "price", "yield"] as const).map((key) => <th key={key} className={cn("px-4 py-3 font-medium", key !== "district" && key !== "yield" && "text-right")}>{t(`rents.col.${key}`)}</th>)}
                <th className="px-4 py-3" />
              </tr></thead>
              <tbody className="divide-y divide-slate-100">{districts.map((district) => {
                const active = district.district === state.district;
                return <tr key={district.district} onClick={() => select(district.district)} aria-selected={active}
                  className={cn("cursor-pointer hover:bg-slate-50", active && "bg-brand-50/60 shadow-[inset_3px_0_0_#1f6f78]")}>
                  <td className="px-4 py-3 font-semibold text-slate-900">{district.district}</td>
                  <td className="px-4 py-3 text-right tabular-nums">{district.rentals.toLocaleString()}</td>
                  <td className="px-4 py-3 text-right tabular-nums">{formatRent(district.medianRentPerPing)}</td>
                  <td className="px-4 py-3 text-right tabular-nums">{wan(district.medianMonthlyRent)}</td>
                  <td className="px-4 py-3 text-right tabular-nums">{district.sales.toLocaleString()}</td>
                  <td className="px-4 py-3 text-right tabular-nums">{wan(district.medianPricePerPing)}</td>
                  <td className="px-4 py-3"><YieldBar value={district.grossYield} max={maxYield} color={district.grossYield === null ? null : PRICE_CLASSES[priceClass(district.grossYield, breaks)].hex} /></td>
                  <td className="px-4 py-3 text-right"><Sample district={district} t={t} /></td>
                </tr>;
              })}</tbody>
            </table></div>
            <div className="divide-y divide-slate-100 md:hidden">{districts.map((district) => <article key={district.district} onClick={() => select(district.district)}
              className={cn("space-y-3 p-4", district.district === state.district && "bg-brand-50/60")}>
              <div className="flex items-center justify-between"><h3 className="font-semibold text-slate-900">{district.district}</h3><Sample district={district} t={t} /></div>
              <div className="grid grid-cols-2 gap-3 text-xs">
                <Value label={t("rents.col.rent")} value={formatRent(district.medianRentPerPing)} /><Value label={t("rents.col.yield")} value={formatYield(district.grossYield)} />
                <Value label={t("rents.col.monthly")} value={wan(district.medianMonthlyRent)} /><Value label={t("rents.col.price")} value={wan(district.medianPricePerPing)} />
                <Value label={t("rents.col.rentals")} value={district.rentals.toLocaleString()} /><Value label={t("rents.col.sales")} value={district.sales.toLocaleString()} />
              </div>
            </article>)}</div>
          </>}
        </section>

        <aside className="rounded-2xl border border-slate-200 bg-white p-5 text-xs leading-6 text-slate-600">
          <h2 className="text-sm font-semibold text-slate-900">{t("rents.methodTitle")}</h2>
          <ul className="mt-2 list-disc space-y-1 pl-4">
            <li>{t("rents.method.leases")}</li>
            <li>{t("rents.method.inferred")}</li>
            <li>{t("rents.method.yield")}</li>
          </ul>
        </aside>
      </>}
    </div>
  </main>;
}

function Metric({ title, value, unit, note, highlight }: { title: string; value: string; unit?: string; note: string; highlight?: boolean }) {
  return <div className={cn("rounded-2xl border bg-white p-4", highlight ? "border-brand-200" : "border-slate-200")}>
    <p className="text-xs font-medium text-slate-500">{title}</p>
    <p className="mt-2 flex items-baseline gap-1.5"><span className={cn("text-2xl font-semibold tabular-nums", highlight ? "text-brand-700" : "text-slate-900")}>{value}</span>{unit && <span className="text-xs text-slate-500">{unit}</span>}</p>
    <p className="mt-1 text-xs text-slate-500">{note}</p>
  </div>;
}

function YieldBar({ value, max, color }: { value: number | null; max: number; color: string | null }) {
  if (value === null || color === null) return <span className="text-slate-400">—</span>;
  return <div className="flex items-center gap-2">
    <span className="w-12 font-semibold tabular-nums text-slate-900">{formatYield(value)}</span>
    <span className="h-1.5 w-28 overflow-hidden rounded-full bg-slate-100"><span className="block h-full rounded-full" style={{ width: `${max > 0 ? value / max * 100 : 0}%`, backgroundColor: color }} /></span>
  </div>;
}

function Sample({ district, t }: { district: RentalDistrict; t: (key: string) => string }) {
  const sufficient = district.grossYield !== null;
  return <span className={cn("inline-flex rounded-full px-2 py-1 text-[11px] font-medium", sufficient ? "bg-emerald-50 text-emerald-700" : "bg-amber-50 text-amber-700")}>
    {sufficient ? t("rents.sample.ok") : t("rents.lowSample")}
  </span>;
}

function HoverRow({ label, value }: { label: string; value: string }) {
  return <div className="flex justify-between gap-2"><dt className="text-slate-500">{label}</dt><dd className="tabular-nums text-slate-800">{value}</dd></div>;
}

function Value({ label, value }: { label: string; value: string }) {
  return <div><p className="text-slate-500">{label}</p><p className="mt-0.5 font-semibold tabular-nums text-slate-900">{value}</p></div>;
}
