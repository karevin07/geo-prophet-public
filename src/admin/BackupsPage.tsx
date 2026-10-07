import { HardDrive, Info, Undo2 } from "lucide-react";
import type { SystemStatus } from "../lib/api/admin";
import { useLanguage } from "../lib/i18n";
import { cn, fill } from "../lib/utils";
import { diskShare, diskTone } from "./health";
import { BackupList } from "./OverviewPage";
import { CARD, formatBytes } from "./shared";

/** Every kept dump, the space left for more, and how to restore one. */
export default function BackupsPage({ system }: { system: SystemStatus }) {
  const { t } = useLanguage();
  const share = diskShare(system);
  const tone = diskTone(system);
  return (
    <div className="space-y-5">
      <section className={cn(CARD, "p-4 sm:p-5")}>
        <h2 className="text-base font-semibold text-slate-900">{t("admin.backups.all")}</h2>
        <BackupList system={system} />
      </section>

      <div className="grid gap-5 lg:grid-cols-2">
        <section className={cn(CARD, "p-4 sm:p-5")}>
          <h2 className="flex items-center gap-2 text-base font-semibold text-slate-900"><HardDrive className="h-4 w-4 text-brand-600" />{t("admin.backups.disk")}</h2>
          {system.disk && share != null ? (
            <>
              <div className={cn("mt-3 text-2xl font-semibold tabular-nums", tone === "red" ? "text-red-600" : tone === "amber" ? "text-amber-600" : "text-slate-900")}>{Math.round(share * 100)}%</div>
              <div className="mt-2 h-2 rounded-full bg-slate-100">
                <div className={cn("h-full rounded-full", tone === "red" ? "bg-red-500" : tone === "amber" ? "bg-amber-400" : "bg-brand-600")} style={{ width: `${share * 100}%` }} />
              </div>
              <p className="mt-2 text-xs text-slate-500">{fill(t("admin.card.diskDetail"), { used: formatBytes(system.disk.usedBytes), total: formatBytes(system.disk.totalBytes), free: formatBytes(system.disk.freeBytes) })}</p>
              {tone !== "green" && <p className="mt-2 text-xs text-amber-700">{t("admin.problem.diskHint")}</p>}
            </>
          ) : <p className="mt-3 text-sm text-slate-500">{t("admin.card.noBackupDir")}</p>}
        </section>

        <section className={cn(CARD, "space-y-3 p-4 sm:p-5")}>
          <h2 className="flex items-center gap-2 text-base font-semibold text-slate-900"><Undo2 className="h-4 w-4 text-brand-600" />{t("admin.backups.restore")}</h2>
          <p className="text-sm leading-relaxed text-slate-600">{t("admin.backups.restoreSteps")}</p>
          <pre className="overflow-x-auto rounded-lg bg-slate-900 px-3 py-2 font-mono text-xs text-slate-100">make restore FILE={system.backups[0]?.name ?? "geoprophet-<UTC>.dump"} CONFIRM=yes</pre>
          <p className="flex items-start gap-2 text-xs text-slate-500"><Info className="mt-0.5 h-3.5 w-3.5 shrink-0" />{t("admin.backups.offsite")}</p>
        </section>
      </div>
    </div>
  );
}
