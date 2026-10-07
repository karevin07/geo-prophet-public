import { useEffect, useId, useState } from "react";
import { Info, RotateCw } from "lucide-react";
import type { InvestmentLiquidityArea, InvestmentLiquidityReport, InvestmentQuery, TransactionAPI } from "../../lib/api";
import { useLanguage } from "../../lib/i18n";
import { cn } from "../../lib/utils";

interface Props { api: TransactionAPI; query: InvestmentQuery }

function percentage(value: number | null) {
  if (value === null) return "—";
  return `${value > 0 ? "+" : ""}${(value * 100).toFixed(1)}%`;
}

function monthBefore(month: string) {
  const [year, value] = month.split("-").map(Number);
  return new Date(Date.UTC(year, value - 1, 0)).toISOString().slice(0, 7);
}

export default function LiquidityResearch({ api, query }: Props) {
  const { t } = useLanguage();
  const [report, setReport] = useState<InvestmentLiquidityReport | null>(null);
  const [selected, setSelected] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [retry, setRetry] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true); setError(false); setReport(null); setSelected([]);
    api.investmentLiquidity(query, controller.signal).then((value) => {
      if (controller.signal.aborted) return;
      setReport(value);
      setSelected(value.areas.slice(0, 3).map((area) => area.district));
    }).catch(() => {
      if (!controller.signal.aborted) setError(true);
    }).finally(() => {
      if (!controller.signal.aborted) setLoading(false);
    });
    return () => controller.abort();
  }, [api, query, retry]);

  const compared = selected.flatMap((district) => report?.areas.filter((area) => area.district === district) ?? []);
  // The same vertical scale makes district-to-district volumes comparable.
  const maximum = Math.max(1, ...compared.flatMap((area) => area.monthly.map((month) => month.count)));
  const toggle = (district: string) => setSelected((current) => current.includes(district)
    ? current.filter((value) => value !== district) : current.length < 3 ? [...current, district] : current);

  return <section aria-label={t("liquidity.title")} className="space-y-4 rounded-2xl border border-slate-200 bg-white p-5 shadow-[0_4px_16px_rgba(15,23,42,0.04)]">
    <header>
      <h2 className="text-lg font-semibold text-slate-900">{t("liquidity.title")}</h2>
      <p className="mt-1 text-sm text-slate-600">{t("liquidity.subtitle")}</p>
    </header>
    <div className="flex gap-2 rounded-xl bg-slate-50 p-3 text-xs leading-5 text-slate-600">
      <Info className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" /><p>{t("liquidity.caution")}</p>
    </div>
    {loading && <p role="status" className="text-sm text-slate-500">{t("status.loading")}</p>}
    {error && <div role="alert" className="flex flex-wrap items-center justify-between gap-3 text-sm text-amber-800">
      <p>{t("liquidity.error")}</p>
      <button type="button" onClick={() => setRetry((value) => value + 1)} className="inline-flex items-center gap-1 rounded-lg border border-amber-200 px-3 py-1.5 font-medium"><RotateCw className="h-4 w-4" aria-hidden="true" />{t("investment.retry")}</button>
    </div>}
    {report && !report.through && <p className="text-sm text-slate-500">{t("liquidity.noCoverage")}</p>}
    {report?.through && <>
      <p className="text-xs text-slate-500">{t("liquidity.current")}: {report.currentFrom} – {report.through} · {t("liquidity.previous")}: {report.previousFrom} – {monthBefore(report.currentFrom)}</p>
      {report.incompleteMonths.length > 0 && <div className="rounded-xl border border-amber-200 bg-amber-50 p-3 text-xs leading-5 text-amber-900">
        <p>{t("liquidity.coverageWarning")}</p><p className="mt-1">{t("liquidity.incompleteMonths")}: {report.incompleteMonths.join("、")}</p>
      </div>}
      {report.areas.length === 0 ? <p className="text-sm text-slate-500">{t("investment.empty")}</p> : <>
        <p className="text-xs text-slate-500">{t("liquidity.select")}</p>
        <div className="flex flex-wrap gap-2" role="group" aria-label={t("liquidity.select")}>
          {report.areas.map((area) => <button key={area.district} type="button" onClick={() => toggle(area.district)} aria-pressed={selected.includes(area.district)} disabled={!selected.includes(area.district) && selected.length >= 3}
            className={cn("rounded-full border px-3 py-1.5 text-sm transition disabled:opacity-40", selected.includes(area.district) ? "border-brand-600 bg-brand-50 font-medium text-brand-700" : "border-slate-200 text-slate-600 hover:border-brand-300")}>
            {area.district} <span className="text-xs tabular-nums opacity-70">{area.currentCount.toLocaleString()}</span>
          </button>)}
        </div>
        {compared.length === 0 && <p className="text-sm text-slate-500">{t("liquidity.choose")}</p>}
        <div className="grid gap-4 lg:grid-cols-3">
          {compared.map((area) => <article key={area.district} className="min-w-0 rounded-xl border border-slate-200 p-4">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <h3 className="font-semibold text-slate-900">{area.district}</h3>
              <span className={cn("rounded-full px-2 py-1 text-[11px]", area.sufficientCurrentSample && report.currentCoverageComplete ? "bg-brand-50 text-brand-700" : "bg-amber-50 text-amber-800")}>
                {!report.currentCoverageComplete ? t("liquidity.coverageGap") : !area.sufficientCurrentSample ? t("investment.evidence.low") : t("liquidity.sampleReady")}
              </span>
            </div>
            <dl className="mt-4 grid grid-cols-2 gap-x-3 gap-y-4">
              <Metric label={t("liquidity.sales")} value={area.currentCount.toLocaleString()} note={`${t("liquidity.priorSales")}: ${area.previousCount.toLocaleString()}`} />
              <Metric label={t("liquidity.activeMonths")} value={area.activeMonths === null ? "—" : `${area.activeMonths} / 12`} note={t("liquidity.activeHint")} />
              <Metric label={t("liquidity.zeroRun")} value={area.longestZeroRun === null ? "—" : `${area.longestZeroRun}`} note={t("liquidity.zeroHint")} />
              <Metric label={t("liquidity.volumeChange")} value={percentage(area.volumeChange)} note={t("liquidity.changeHint")} />
            </dl>
            <div className="mt-5 border-t border-slate-100 pt-3">
              <h4 className="text-xs font-medium text-slate-600">{t("liquidity.monthly")}</h4>
              <MonthlyBars area={area} maximum={maximum} currentFrom={report.currentFrom} />
            </div>
          </article>)}
        </div>
        {compared.length > 0 && <>
          <div className="flex flex-wrap gap-4 text-xs text-slate-500"><span className="inline-flex items-center gap-1.5"><span className="h-2.5 w-2.5 rounded-sm bg-slate-300" />{t("liquidity.previous")}</span><span className="inline-flex items-center gap-1.5"><span className="h-2.5 w-2.5 rounded-sm bg-brand-600" />{t("liquidity.current")}</span><span>{t("liquidity.sharedScale")}</span></div>
          <details className="rounded-lg border border-slate-200 p-3">
            <summary className="cursor-pointer text-sm font-medium text-slate-700">{t("liquidity.monthlyTable")}</summary>
            <div className="mt-3 overflow-x-auto"><table className="w-full min-w-[340px] text-left text-xs">
              <caption className="sr-only">{t("liquidity.monthlyTable")}</caption>
              <thead><tr className="border-b border-slate-200"><th scope="col" className="py-2 pr-3">{t("liquidity.month")}</th><th scope="col" className="py-2 pr-3">{t("liquidity.coverage")}</th>{compared.map((area) => <th key={area.district} scope="col" className="py-2 pr-3">{area.district}</th>)}</tr></thead>
              <tbody>{compared[0].monthly.map((month, index) => <tr key={month.month} className={cn("border-b border-slate-100", month.month >= report.currentFrom && "bg-brand-50/40")}>
                <th scope="row" className="py-2 pr-3 font-normal tabular-nums">{month.month}</th><td className="py-2 pr-3">{month.coverageComplete ? t("liquidity.covered") : t("liquidity.coverageGap")}</td>
                {compared.map((area) => <td key={area.district} className="py-2 pr-3 tabular-nums">{area.monthly[index]?.count.toLocaleString() ?? "—"}</td>)}
              </tr>)}</tbody>
            </table></div>
          </details>
        </>}
      </>}
      <p className="text-xs leading-5 text-slate-500">{t("liquidity.method").replace("{minimum}", String(report.minimumSample))}</p>
    </>}
  </section>;
}

function Metric({ label, value, note }: { label: string; value: string; note: string }) {
  return <div><dt className="text-xs text-slate-500">{label}</dt><dd className="mt-1 text-xl font-semibold tabular-nums text-slate-900">{value}</dd><dd className="mt-1 text-[11px] leading-4 text-slate-500">{note}</dd></div>;
}

function MonthlyBars({ area, maximum, currentFrom }: { area: InvestmentLiquidityArea; maximum: number; currentFrom: string }) {
  const { t } = useLanguage();
  const [focusedMonth, setFocusedMonth] = useState<string | null>(null);
  const focused = area.monthly.find((month) => month.month === focusedMonth);
  const height = 100, step = 12, left = 30, base = 114;
  const patternID = useId();
  return <>
    <svg className="mt-2 w-full" viewBox="0 0 324 138" role="img" aria-label={`${area.district} · ${t("liquidity.monthly")}`}>
      <defs><pattern id={patternID} width="5" height="5" patternUnits="userSpaceOnUse"><path d="M0 5L5 0" stroke="#d97706" strokeOpacity="0.2" /></pattern></defs>
      <line x1={left} x2="318" y1={base} y2={base} stroke="#e2e8f0" />
      <line x1={left} x2="318" y1={base - height} y2={base - height} stroke="#f1f5f9" />
      <text x="24" y="18" textAnchor="end" fontSize="9" fill="#64748b">{maximum}</text><text x="24" y={base + 3} textAnchor="end" fontSize="9" fill="#64748b">0</text>
      {area.monthly.map((month, index) => {
        const barHeight = Math.max(1, month.count / maximum * height), x = left + index * step;
        return <g key={month.month} tabIndex={0} role="img" aria-label={`${month.month}: ${month.count} · ${month.coverageComplete ? t("liquidity.covered") : t("liquidity.coverageGap")}`}
          className="cursor-default outline-none" onMouseEnter={() => setFocusedMonth(month.month)} onMouseLeave={() => setFocusedMonth(null)} onFocus={() => setFocusedMonth(month.month)} onBlur={() => setFocusedMonth(null)}>
          <rect x={x} y={base - height} width="10" height={height} fill={month.coverageComplete ? "transparent" : `url(#${patternID})`} stroke={focusedMonth === month.month ? "#1f6f78" : "none"} />
          <rect x={x} y={base - barHeight} width="10" height={barHeight} rx="1" fill={month.month >= currentFrom ? "#1f6f78" : "#cbd5e1"} />
        </g>;
      })}
      <text x={left} y="132" fontSize="9" fill="#64748b">{area.monthly[0]?.month}</text><text x={left + 12 * step} y="132" fontSize="9" fill="#64748b">{currentFrom}</text><text x="318" y="132" textAnchor="end" fontSize="9" fill="#64748b">{area.monthly.at(-1)?.month}</text>
    </svg>
    <p aria-live="polite" className="min-h-8 text-[11px] leading-4 text-slate-500">{focused ? `${focused.month} · ${focused.count} ${t("liquidity.countUnit")} · ${focused.coverageComplete ? t("liquidity.covered") : t("liquidity.coverageGap")}` : t("liquidity.chartHint")}</p>
  </>;
}
