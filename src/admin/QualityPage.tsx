import type { ReactNode } from "react";
import { AlertTriangle, CheckCircle2, Database, MapPin, Rows3 } from "lucide-react";
import type { DataStatus, QualityAlert, QualityMetric } from "../lib/api";
import { CITY_NAMES, periodLabel } from "../lib/data-status";
import { useLanguage } from "../lib/i18n";
import { cn } from "../lib/utils";
import { CARD, shortTime } from "./shared";

const percent = (value: number | null) => value == null ? "—" : `${(value * 100).toFixed(1)}%`;
const count = (value: number) => value.toLocaleString("zh-TW");

/** The layout follows the GeoProphet Stitch quality-screen concept, using only real API values. */
export default function QualityPage({ status }: { status: DataStatus }) {
  const { t } = useLanguage();
  const quality = status.quality;
  const metrics = quality?.metrics ?? [];
  const alerts = quality?.alerts ?? [];
  const releases = metrics.filter((metric) => metric.dataset === "sales_release");
  const latestRelease = releases.map((metric) => metric.season).sort().at(-1);
  const current = releases.filter((metric) => metric.season === latestRelease);
  const processed = current.reduce((sum, metric) => sum + metric.rowsRead, 0);
  const precise = current.reduce((sum, metric) => sum + metric.preciseCount, 0);
  const written = current.reduce((sum, metric) => sum + metric.rowsUpserted, 0);

  return (
    <div className="space-y-5">
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <Kpi icon={<Database />} label={t("quality.kpi.release")} value={latestRelease ? periodLabel(latestRelease) : "—"} />
        <Kpi icon={<Rows3 />} label={t("quality.kpi.read")} value={current.length ? count(processed) : "—"} note={t("quality.kpi.readNote")} />
        <Kpi icon={<MapPin />} label={t("quality.kpi.precise")} value={written && current.every((metric) => metric.preciseRate != null) ? percent(precise / written) : "—"} note={t("quality.kpi.preciseNote")} />
        <Kpi icon={<AlertTriangle />} label={t("quality.kpi.alerts")} value={count(alerts.length)} alert={alerts.length > 0} />
      </div>

      <section className={cn(CARD, "p-4 sm:p-5")}>
        <h2 className="text-base font-semibold text-slate-900">{t("quality.table.title")}</h2>
        <p className="mt-1 text-xs text-slate-500">{t("quality.table.note")}</p>
        {metrics.length === 0 ? <p className="py-8 text-center text-sm text-slate-500">{t("quality.empty")}</p> : (
          <div className="mt-3 overflow-x-auto">
            <table className="w-full min-w-[56rem] text-left text-sm">
              <thead className="border-b border-slate-200 text-xs text-slate-500">
                <tr>
                  {["quality.col.city", "quality.col.dataset", "quality.col.period", "quality.col.read", "quality.col.skipped", "quality.col.written", "quality.col.located", "quality.col.precise", "quality.col.change", "quality.col.finished"].map((key) => (
                    <th key={key} className="px-2 py-2 font-medium">{t(key)}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {metrics.map((metric) => <MetricRow key={`${metric.dataset}-${metric.cityCode}`} metric={metric} />)}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <section className={cn(CARD, "p-4 sm:p-5")}>
        <h2 className="text-base font-semibold text-slate-900">{t("quality.alerts.title")}</h2>
        <p className="mt-1 text-xs text-slate-500">{t("quality.alerts.note")}</p>
        {alerts.length === 0 ? (
          <p className="mt-4 flex items-center gap-2 text-sm text-emerald-700"><CheckCircle2 className="h-4 w-4" />{t("quality.alerts.none")}</p>
        ) : (
          <ul className="mt-3 divide-y divide-slate-100">
            {alerts.map((alert) => <AlertRow key={`${alert.runId}-${alert.code}`} alert={alert} />)}
          </ul>
        )}
      </section>
    </div>
  );
}

function Kpi({ icon, label, value, note, alert = false }: { icon: ReactNode; label: string; value: string; note?: string; alert?: boolean }) {
  return <div className={cn(CARD, "px-4 py-4")}>
    <div className="flex items-center justify-between text-xs text-slate-500">{label}<span className="[&>svg]:h-4 [&>svg]:w-4">{icon}</span></div>
    <strong className={cn("mt-2 block text-xl font-semibold tabular-nums", alert ? "text-amber-700" : "text-slate-900")}>{value}</strong>
    {note && <p className="mt-2 text-xs text-slate-500">{note}</p>}
  </div>;
}

function MetricRow({ metric }: { metric: QualityMetric }) {
  const { t } = useLanguage();
  return <tr className="border-b border-slate-100 last:border-0">
    <td className="whitespace-nowrap px-2 py-3 font-medium">{CITY_NAMES[metric.cityCode] ?? metric.cityCode}</td>
    <td className="whitespace-nowrap px-2 py-3">{t(`data.dataset.${metric.dataset}`)}</td>
    <td className="whitespace-nowrap px-2 py-3">{periodLabel(metric.season)}</td>
    <td className="px-2 py-3 tabular-nums">{count(metric.rowsRead)}</td>
    <td className="px-2 py-3 tabular-nums">{count(metric.rowsSkipped)}</td>
    <td className="px-2 py-3 tabular-nums">{count(metric.rowsUpserted)}</td>
    <td className="px-2 py-3 tabular-nums">{percent(metric.locatedRate)}</td>
    <td className="px-2 py-3 tabular-nums">{percent(metric.preciseRate)}</td>
    <td className={cn("px-2 py-3 tabular-nums", metric.volumeChange != null && metric.volumeChange < -0.35 && "text-amber-700")}>
      {metric.volumeChange == null ? "—" : `${metric.volumeChange > 0 ? "+" : ""}${percent(metric.volumeChange)}`}
    </td>
    <td className="whitespace-nowrap px-2 py-3 text-slate-500">{shortTime(metric.finishedAt)}</td>
  </tr>;
}

function AlertRow({ alert }: { alert: QualityAlert }) {
  const { t } = useLanguage();
  const error = alert.severity === "error";
  return <li className="flex gap-3 py-3 text-sm">
    <AlertTriangle className={cn("mt-0.5 h-4 w-4 shrink-0", error ? "text-red-600" : "text-amber-600")} />
    <div>
      <p className="font-medium text-slate-800">{CITY_NAMES[alert.cityCode] ?? alert.cityCode} · {t(`data.dataset.${alert.dataset}`)} · {periodLabel(alert.season)}</p>
      <p className="mt-0.5 text-slate-600">{t(`quality.alert.${alert.code}`)}</p>
    </div>
  </li>;
}
