import type { ReactNode } from "react";
import { AlertCircle, AlertTriangle, ArrowRight, Info } from "lucide-react";
import type { DataStatus } from "../lib/api";
import type { SystemStatus } from "../lib/api/admin";
import { CITY_NAMES, periodLabel } from "../lib/data-status";
import { useLanguage } from "../lib/i18n";
import { cn, fill } from "../lib/utils";
import { backupTone, diskShare, diskTone, problems, syncTone, type Problem } from "./health";
import { CARD, Dot, formatBytes, shortTime, type Tone } from "./shared";

interface Props {
  status: DataStatus;
  system: SystemStatus;
  onNavigate: (page: "sync" | "backups") => void;
}

/** Health at a glance: four status cards, open problems, database size and backups. */
export default function OverviewPage({ status, system, onNavigate }: Props) {
  const { t } = useLanguage();
  const now = new Date();
  const lastRun = status.recentRuns.find((run) => run.status === "succeeded" && run.finishedAt);
  const release = status.lastSuccessfulRuns.filter((run) => run.dataset === "sales_release").map((run) => run.season).sort().at(-1);
  const newest = system.backups[0];
  const share = diskShare(system);
  const found = problems(status, system, now);
  const tables = system.database.tables;
  const shown = tables.slice(0, 3);
  const rest = tables.slice(3).reduce((sum, table) => sum + table.bytes, 0);

  return (
    <div className="space-y-5">
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <StatusCard title={t("admin.card.sync")} tone={syncTone(status)} onClick={() => onNavigate("sync")}
          lines={[fill(t("admin.card.syncDetail"), { time: shortTime(lastRun?.finishedAt) }), release && fill(t("admin.card.release"), { period: periodLabel(release) })]} />
        <StatusCard title={t("admin.card.backup")} tone={backupTone(system, now)} onClick={() => onNavigate("backups")}
          lines={!system.disk ? [t("admin.card.noBackupDir")] : newest ? [
            fill(t("admin.card.backupDetail"), { time: shortTime(newest.modifiedAt), size: formatBytes(newest.bytes) }),
            fill(t("admin.card.backupCount"), { count: system.backups.length, keep: system.backupKeep }),
          ] : [t("admin.problem.noBackup")]} />
        <StatusCard title={t("admin.card.disk")} tone={diskTone(system)} onClick={() => onNavigate("backups")}
          big={share == null ? "—" : `${Math.round(share * 100)}%`}
          lines={system.disk ? [fill(t("admin.card.diskDetail"), { used: formatBytes(system.disk.usedBytes), total: formatBytes(system.disk.totalBytes), free: formatBytes(system.disk.freeBytes) })] : [t("admin.card.noBackupDir")]}
          bar={share} />
        <StatusCard title={t("admin.card.database")} tone="green" toneLabel="PostgreSQL"
          big={formatBytes(system.database.bytes)}
          lines={[fill(t("admin.card.databaseDetail"), { count: system.database.transactions.toLocaleString("zh-TW"), tables: tables.length })]} />
      </div>

      <div className="grid gap-5 lg:grid-cols-[3fr_2fr]">
        <section className={cn(CARD, "p-4 sm:p-5")}>
          <h2 className="text-base font-semibold text-slate-900">{t("admin.problems")}</h2>
          {found.length === 0
            ? <p className="py-8 text-center text-sm text-slate-500">{t("admin.problems.none")}</p>
            : <ul className="mt-3 divide-y divide-slate-100">{found.map((problem, index) => <ProblemItem key={index} problem={problem} />)}</ul>}
        </section>

        <section className={cn(CARD, "p-4 sm:p-5")}>
          <div className="flex items-baseline justify-between gap-2">
            <h2 className="text-base font-semibold text-slate-900">{t("admin.database")}</h2>
            <span className="text-xs tabular-nums text-slate-500">{fill(t("admin.database.total"), { size: formatBytes(system.database.bytes) })}</span>
          </div>
          <ul className="mt-4 space-y-4">
            {[...shown.map((table) => [table.name, table.bytes] as const), ...(rest > 0 ? [["…", rest] as const] : [])].map(([name, bytes]) => (
              <li key={name}>
                <div className="flex justify-between gap-2 text-sm">
                  <span className="truncate font-mono text-xs text-slate-700">{name}</span>
                  <span className="shrink-0 tabular-nums text-slate-500">{formatBytes(bytes)}</span>
                </div>
                <div className="mt-1.5 h-1.5 rounded-full bg-slate-100">
                  <div className="h-full rounded-full bg-brand-600" style={{ width: `${Math.max(1, (bytes / system.database.bytes) * 100)}%` }} />
                </div>
              </li>
            ))}
          </ul>
        </section>
      </div>

      <section className={cn(CARD, "p-4 sm:p-5")}>
        <div className="flex items-center justify-between gap-2">
          <h2 className="text-base font-semibold text-slate-900">{t("admin.backups.recent")}</h2>
          <button onClick={() => onNavigate("backups")} className="flex items-center gap-1 text-xs font-medium text-brand-700">{t("admin.backups.all")}<ArrowRight className="h-3.5 w-3.5" /></button>
        </div>
        <BackupList system={system} limit={3} />
      </section>
    </div>
  );
}

function StatusCard({ title, tone, toneLabel, big, lines, bar, onClick }: {
  title: string; tone: Tone; toneLabel?: string; big?: string; lines: (string | undefined | false)[]; bar?: number | null; onClick?: () => void;
}) {
  const { t } = useLanguage();
  const body = (
    <>
      <div className="flex items-center gap-2 text-sm font-semibold text-slate-800">
        <Dot tone={tone} />{title}<span className="font-normal text-slate-400">· {toneLabel ?? t(`admin.tone.${tone}`)}</span>
      </div>
      {big && <div className={cn("mt-2 text-2xl font-semibold tabular-nums", tone === "amber" ? "text-amber-600" : tone === "red" ? "text-red-600" : "text-slate-900")}>{big}</div>}
      {bar != null && (
        <div className="mt-2 h-1.5 rounded-full bg-slate-100">
          <div className={cn("h-full rounded-full", tone === "red" ? "bg-red-500" : tone === "amber" ? "bg-amber-400" : "bg-brand-600")} style={{ width: `${Math.min(100, bar * 100)}%` }} />
        </div>
      )}
      <div className="mt-2 space-y-0.5 text-xs text-slate-500">{lines.filter(Boolean).map((line) => <p key={String(line)}>{line}</p>)}</div>
    </>
  );
  return onClick
    ? <button onClick={onClick} className={cn(CARD, "px-4 py-4 text-left transition hover:border-slate-300")}>{body}</button>
    : <div className={cn(CARD, "px-4 py-4")}>{body}</div>;
}

function ProblemItem({ problem }: { problem: Problem }) {
  const { t } = useLanguage();
  const Icon = problem.severity === "error" ? AlertCircle : problem.severity === "warning" ? AlertTriangle : Info;
  const color = problem.severity === "error" ? "text-red-600" : problem.severity === "warning" ? "text-amber-500" : "text-slate-400";
  const run = problem.run;
  const values: Record<string, string | number> = { ...problem.values };
  if (run) {
    values.time = shortTime(run.startedAt);
    values.recoveredAt = shortTime(String(problem.values.recoveredAt));
    values.dataset = t(`data.dataset.${run.dataset}`);
    values.city = run.cityCode ? `（${CITY_NAMES[run.cityCode] ?? run.cityCode}）` : "";
  }
  let detail: ReactNode = null;
  if (problem.kind === "disk") detail = <p className="mt-1 text-xs text-slate-500">{t("admin.problem.diskHint")}</p>;
  if (run?.error) detail = <pre className="mt-2 overflow-x-auto whitespace-pre-wrap break-all rounded-lg bg-slate-900 px-3 py-2 font-mono text-xs leading-relaxed text-slate-100">{run.error}</pre>;
  return (
    <li className="flex gap-3 py-3">
      <Icon className={cn("mt-0.5 h-4 w-4 shrink-0", color)} />
      <div className="min-w-0 flex-1">
        <p className="text-sm text-slate-800">{fill(t(`admin.problem.${problem.kind}`), values)}</p>
        {detail}
      </div>
    </li>
  );
}

export function BackupList({ system, limit }: { system: SystemStatus; limit?: number }) {
  const { t } = useLanguage();
  const backups = limit ? system.backups.slice(0, limit) : system.backups;
  if (!system.disk) return <p className="py-6 text-center text-sm text-slate-500">{t("admin.card.noBackupDir")}</p>;
  if (backups.length === 0) return <p className="py-6 text-center text-sm text-slate-500">{t("admin.backups.empty")}</p>;
  return (
    <div className="-mx-4 mt-3 overflow-x-auto px-4 sm:mx-0 sm:px-0">
      <table className="w-full min-w-[28rem] text-left text-sm">
        <thead>
          <tr className="border-b border-slate-100 text-xs text-slate-500">
            <th className="py-2 font-medium">{t("admin.backups.col.name")}</th>
            <th className="py-2 font-medium">{t("admin.backups.col.time")}</th>
            <th className="py-2 text-right font-medium">{t("admin.backups.col.size")}</th>
          </tr>
        </thead>
        <tbody>
          {backups.map((backup) => (
            <tr key={backup.name} className="border-b border-slate-100 last:border-0">
              <td className="py-2.5 font-mono text-xs text-slate-700">{backup.name}</td>
              <td className="py-2.5 tabular-nums text-slate-600">{shortTime(backup.modifiedAt)}</td>
              <td className="py-2.5 text-right tabular-nums text-slate-600">{formatBytes(backup.bytes)}</td>
            </tr>
          ))}
        </tbody>
      </table>
      <p className="mt-3 text-xs text-slate-400">{fill(t("admin.backups.note"), { hours: system.backupIntervalHours, keep: system.backupKeep })}</p>
    </div>
  );
}
