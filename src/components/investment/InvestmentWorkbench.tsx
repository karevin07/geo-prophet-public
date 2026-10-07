import { useEffect, useState } from "react";
import type { InvestmentQuery, InvestmentWorkbenchReport, TransactionAPI } from "../../lib/api";
import { useLanguage } from "../../lib/i18n";

interface Props { api: TransactionAPI; query: InvestmentQuery }

/** A simple unlevered annual cash-flow scenario. No tax or appreciation assumptions. */
export function netScenario(price: number, monthlyRent: number, vacancyMonths: number, annualCosts: number, acquisitionCosts: number) {
  if (![price, monthlyRent, vacancyMonths, annualCosts, acquisitionCosts].every(Number.isFinite) || price <= 0 || monthlyRent < 0 || vacancyMonths < 0 || vacancyMonths > 12 || annualCosts < 0 || acquisitionCosts < 0) return null;
  const annualCash = monthlyRent * (12 - vacancyMonths) - annualCosts;
  return { annualCash, yield: annualCash / (price + acquisitionCosts) };
}

function money(value: number | null) { return value === null ? "—" : new Intl.NumberFormat("zh-TW", { maximumFractionDigits: 0 }).format(value); }
function percent(value: number | null) { return value === null ? "—" : `${(value * 100).toFixed(1)}%`; }

export default function InvestmentWorkbench({ api, query }: Props) {
  const { t } = useLanguage();
  const [report, setReport] = useState<InvestmentWorkbenchReport | null>(null);
  const [error, setError] = useState(false);
  const [loading, setLoading] = useState(true);
  const [selected, setSelected] = useState<string[]>([]);
  const [price, setPrice] = useState("");
  const [rent, setRent] = useState("");
  const [vacancy, setVacancy] = useState("");
  const [annualCosts, setAnnualCosts] = useState("");
  const [acquisitionCosts, setAcquisitionCosts] = useState("");

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true); setError(false); setReport(null); setSelected([]);
    api.investmentWorkbench(query, controller.signal).then((value) => {
      if (!controller.signal.aborted) setReport(value);
    }).catch(() => { if (!controller.signal.aborted) setError(true); }).finally(() => {
      if (!controller.signal.aborted) setLoading(false);
    });
    return () => controller.abort();
  }, [api, query]);

  const scenario = price !== "" && rent !== "" && vacancy !== "" && annualCosts !== "" && acquisitionCosts !== ""
    ? netScenario(Number(price), Number(rent), Number(vacancy), Number(annualCosts), Number(acquisitionCosts)) : null;
  const compared = selected.map((district) => report?.areas.find((area) => area.district === district)).filter((area) => area !== undefined);
  const toggle = (district: string) => setSelected((current) => current.includes(district) ? current.filter((value) => value !== district) : current.length < 3 ? [...current, district] : current);

  return <section className="space-y-4 rounded-2xl border border-slate-200 bg-white p-5 shadow-[0_4px_16px_rgba(15,23,42,0.04)]" aria-label={t("workbench.title")}>
    <div><h2 className="text-lg font-semibold text-slate-900">{t("workbench.title")}</h2><p className="mt-1 text-sm text-slate-600">{t("workbench.subtitle")}</p></div>
    {loading && <p role="status" className="text-sm text-slate-500">{t("status.loading")}</p>}
    {error && <p role="alert" className="text-sm text-amber-700">{t("workbench.unavailable")}</p>}
    {report && !report.through && <p className="text-sm text-slate-600">{t("workbench.noCoverage")}</p>}
    {report?.through && <>
      <p className="text-xs text-slate-500">{report.currentFrom} – {report.through} · {t("workbench.sameCohort")}</p>
      <div>
        <p className="mb-2 text-sm font-medium text-slate-700">{t("workbench.select")}</p>
        <div className="flex flex-wrap gap-2">{report.areas.map((area) => <button key={area.district} type="button" onClick={() => toggle(area.district)} disabled={!selected.includes(area.district) && selected.length >= 3} aria-pressed={selected.includes(area.district)}
          className={`rounded-full border px-3 py-1.5 text-sm disabled:opacity-40 ${selected.includes(area.district) ? "border-brand-600 bg-brand-50 text-brand-700" : "border-slate-200 text-slate-600"}`}>{area.district}</button>)}</div>
      </div>
      {compared.length > 0 && <div className="grid gap-3 lg:grid-cols-3">{compared.map((area) => <article key={area.district} className="rounded-xl border border-slate-200 bg-slate-50 p-4">
        <h3 className="font-semibold text-slate-900">{area.district}</h3>
        <dl className="mt-3 grid grid-cols-2 gap-3 text-xs"><div><dt className="text-slate-500">{t("workbench.sales")}</dt><dd className="mt-1 font-semibold">{area.currentCount}</dd></div><div><dt className="text-slate-500">{t("workbench.salePrice")}</dt><dd className="mt-1 font-semibold">{money(area.medianPricePerPing)}</dd></div><div><dt className="text-slate-500">{t("workbench.rentals")}</dt><dd className="mt-1 font-semibold">{area.rentalCount}</dd></div><div><dt className="text-slate-500">{t("workbench.rent")}</dt><dd className="mt-1 font-semibold">{money(area.medianRentPerPing)}</dd></div><div><dt className="text-slate-500">{t("workbench.change")}</dt><dd className="mt-1 font-semibold">{percent(area.priceChange)}</dd></div><div><dt className="text-slate-500">{t("workbench.gross")}</dt><dd className="mt-1 font-semibold">{percent(area.grossYield)}</dd></div></dl>
      </article>)}</div>}
      <p className="text-xs leading-5 text-slate-500">{t("workbench.method").replace("{sales}", String(report.minimumSales)).replace("{rentals}", String(report.minimumRentals))}</p>
    </>}
    <div className="border-t border-slate-100 pt-4"><h3 className="font-semibold text-slate-900">{t("workbench.scenario")}</h3><p className="mt-1 text-xs text-slate-500">{t("workbench.scenarioHint")}</p>
      <div className="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-5">
        {([["price", price, setPrice], ["monthlyRent", rent, setRent], ["vacancy", vacancy, setVacancy], ["annualCosts", annualCosts, setAnnualCosts], ["acquisitionCosts", acquisitionCosts, setAcquisitionCosts]] as const).map(([key, value, setter]) =>
          <label key={key} className="text-xs font-medium text-slate-600">{t(`workbench.${key}`)}<input className="mt-1 h-10 w-full rounded-lg border border-slate-200 px-3 text-sm" type="number" inputMode="decimal" min="0" max={key === "vacancy" ? "12" : undefined} step={key === "vacancy" ? "0.5" : "1"} value={value} onChange={(event) => setter(event.target.value)} /></label>)}</div>
      <div className="mt-4 rounded-xl bg-slate-50 p-4 text-sm"><span className="text-slate-600">{t("workbench.netCash")}: </span><strong>{scenario ? money(scenario.annualCash) : "—"}</strong><span className="ml-5 text-slate-600">{t("workbench.netYield")}: </span><strong>{scenario ? percent(scenario.yield) : "—"}</strong></div>
      <p className="mt-2 text-xs leading-5 text-slate-500">{t("workbench.scenarioCaution")}</p>
    </div>
  </section>;
}
