import { useCallback, useEffect, useState, type ReactNode } from "react";
import { AlertCircle, ArrowLeft, CalendarClock, DatabaseBackup, LayoutDashboard, Play, RefreshCw, RotateCw, ShieldAlert, ShieldCheck, ScanSearch } from "lucide-react";
import type { DataStatus } from "../lib/api";
import { adminAPI, AdminError, type AdminSession, type SystemStatus } from "../lib/api/admin";
import { useLanguage } from "../lib/i18n";
import { cn, fill } from "../lib/utils";
import BackupsPage from "./BackupsPage";
import OverviewPage from "./OverviewPage";
import { shortTime } from "./shared";
import JobsPage from "./JobsPage";
import OpeningsPage from "./OpeningsPage";
import SyncPage from "./SyncPage";
import QualityPage from "./QualityPage";

const PAGES = {
  overview: { path: "/admin", icon: LayoutDashboard },
  sync: { path: "/admin/sync", icon: RotateCw },
  quality: { path: "/admin/quality", icon: ScanSearch },
  backups: { path: "/admin/backups", icon: DatabaseBackup },
  jobs: { path: "/admin/jobs", icon: Play },
  openings: { path: "/admin/openings", icon: CalendarClock },
} as const;
type Page = keyof typeof PAGES;

function pageOf(pathname: string): Page {
  const path = pathname.replace(/\/+$/, "") || "/admin";
  return (Object.keys(PAGES) as Page[]).find((page) => PAGES[page].path === path) ?? "overview";
}

/** The admin console: allowed Tailscale users only, checked again by every admin API call. */
export default function AdminApp() {
  const { t } = useLanguage();
  const [page, setPage] = useState<Page>(() => pageOf(window.location.pathname));
  const [session, setSession] = useState<AdminSession | null>(null);
  const [denied, setDenied] = useState<"forbidden" | "unavailable" | "error" | null>(null);
  const [status, setStatus] = useState<DataStatus | null>(null);
  const [system, setSystem] = useState<SystemStatus | null>(null);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState(false);
  const [reload, setReload] = useState(0);

  useEffect(() => {
    document.title = `GeoProphet · ${t("admin.title")}`;
    const restore = () => setPage(pageOf(window.location.pathname));
    window.addEventListener("popstate", restore);
    return () => window.removeEventListener("popstate", restore);
  }, [t]);

  useEffect(() => {
    const controller = new AbortController();
    adminAPI.me(controller.signal).then((me) => {
      setSession(me);
      if (!me.admin) setDenied("forbidden");
    }).catch((error) => {
      if (!controller.signal.aborted) setDenied(error instanceof AdminError && error.status === 503 ? "unavailable" : "error");
    });
    return () => controller.abort();
  }, []);

  // Every page shares one snapshot, so moving between pages is instant.
  useEffect(() => {
    if (!session?.admin) return;
    const controller = new AbortController();
    setLoading(true);
    setLoadError(false);
    Promise.all([adminAPI.dataStatus(controller.signal), adminAPI.system(controller.signal)])
      .then(([nextStatus, nextSystem]) => {
        setStatus(nextStatus);
        setSystem(nextSystem);
      })
      .catch((error) => {
        if (controller.signal.aborted) return;
        if (error instanceof AdminError && error.status === 403) setDenied("forbidden");
        else setLoadError(true);
      })
      .finally(() => !controller.signal.aborted && setLoading(false));
    return () => controller.abort();
  }, [session, reload]);

  const reloadData = useCallback(() => setReload((value) => value + 1), []);

  const navigate = useCallback((next: Page) => {
    setPage(next);
    window.history.pushState(null, "", PAGES[next].path);
  }, []);

  if (denied) return <Denied reason={denied} session={session} />;
  if (!session) return <div className="flex h-full items-center justify-center bg-canvas text-sm text-slate-500">{t("admin.loading")}</div>;

  return (
    <div className="flex h-full bg-canvas font-sans text-slate-700">
      <aside className="hidden w-60 shrink-0 flex-col justify-between border-r border-slate-200 bg-white p-4 md:flex">
        <div>
          <div className="mb-6 flex items-center gap-2 px-2">
            <span className="text-base font-semibold text-slate-900">GeoProphet</span>
            <span className="rounded-md bg-brand-50 px-2 py-0.5 text-xs font-semibold text-brand-700">{t("admin.title")}</span>
          </div>
          <nav className="space-y-1" aria-label="admin">
            {(Object.keys(PAGES) as Page[]).map((item) => <NavItem key={item} page={item} active={page === item} onClick={() => navigate(item)} />)}
          </nav>
        </div>
        <div className="space-y-3">
          <UserCard session={session} />
          <a href="/" className="flex items-center gap-1.5 border-t border-slate-100 pt-3 text-xs text-slate-500 hover:text-slate-800">
            <ArrowLeft className="h-3.5 w-3.5" />{t("admin.backToSite")}
          </a>
        </div>
      </aside>

      <main className="min-w-0 flex-1 overflow-y-auto">
        {/* Narrow screens get the pages as a row of tabs instead of the sidebar. */}
        <nav className="flex items-center gap-1 overflow-x-auto border-b border-slate-200 bg-white px-3 py-2 md:hidden" aria-label="admin">
          {(Object.keys(PAGES) as Page[]).map((item) => (
            <button key={item} onClick={() => navigate(item)} aria-current={page === item ? "page" : undefined}
              className={cn("shrink-0 rounded-lg px-3 py-1.5 text-sm", page === item ? "bg-brand-50 font-medium text-brand-700" : "text-slate-600")}>
              {t(`admin.nav.${item}`)}
            </button>
          ))}
          <a href="/" className="ml-auto shrink-0 px-2 text-xs text-slate-500">{t("admin.backToSite")}</a>
        </nav>

        <div className="mx-auto max-w-6xl space-y-5 px-4 py-6 sm:px-8 sm:py-8">
          <header className="flex flex-wrap items-start justify-between gap-3">
            <div>
              <h1 className="text-xl font-semibold text-slate-900 sm:text-2xl">{t(`admin.nav.${page}`)}</h1>
              <p className="mt-1 text-sm text-slate-500">{t(`admin.page.${page}`)}</p>
            </div>
            <div className="flex items-center gap-2 text-xs text-slate-500">
              {status && fill(t("admin.updatedAt"), { time: shortTime(status.generatedAt, true) })}
              <button onClick={() => setReload((value) => value + 1)} aria-label={t("admin.refresh")} title={t("admin.refresh")}
                className="flex h-8 w-8 items-center justify-center rounded-lg border border-slate-200 bg-white text-slate-600 hover:bg-slate-50">
                <RefreshCw className={cn("h-4 w-4", loading && "animate-spin")} />
              </button>
            </div>
          </header>

          {loadError && (
            <div role="alert" className="flex items-center justify-between gap-3 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
              <span className="flex items-center gap-2"><AlertCircle className="h-4 w-4" />{t("admin.loadError")}</span>
              <button onClick={() => setReload((value) => value + 1)} className="font-semibold underline">{t("analysis.retry")}</button>
            </div>
          )}

          {!status || !system ? (loading && <div className="h-96 animate-pulse rounded-2xl bg-slate-100" />) : (
            <div className={cn("transition-opacity", loading && "opacity-60")}>
              {page === "overview" && <OverviewPage status={status} system={system} onNavigate={navigate} />}
              {page === "sync" && <SyncPage status={status} />}
              {page === "quality" && <QualityPage status={status} />}
              {page === "backups" && <BackupsPage system={system} />}
              {page === "jobs" && <JobsPage status={status} system={system} onJobFinished={reloadData} />}
              {page === "openings" && <OpeningsPage />}
            </div>
          )}
        </div>
      </main>
    </div>
  );
}

function NavItem({ page, active, onClick }: { page: Page; active: boolean; onClick: () => void }) {
  const { t } = useLanguage();
  const Icon = PAGES[page].icon;
  return (
    <a href={PAGES[page].path} onClick={(event) => { event.preventDefault(); onClick(); }} aria-current={active ? "page" : undefined}
      className={cn("flex items-center gap-2.5 rounded-lg px-3 py-2.5 text-sm", active ? "bg-brand-50 font-semibold text-brand-700" : "text-slate-600 hover:bg-slate-50 hover:text-slate-900")}>
      <Icon className="h-4 w-4" />{t(`admin.nav.${page}`)}
    </a>
  );
}

function UserCard({ session }: { session: AdminSession }) {
  const { t } = useLanguage();
  const initials = (session.name || session.login).slice(0, 2).toUpperCase();
  return (
    <div className="flex items-center gap-3 px-1">
      {session.profilePic
        ? <img src={session.profilePic} alt="" referrerPolicy="no-referrer" className="h-9 w-9 shrink-0 rounded-full" />
        : <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-slate-100 text-xs font-semibold text-slate-600">{initials}</span>}
      <div className="min-w-0">
        <div className="truncate text-sm font-medium text-slate-900">{session.name || session.login}</div>
        <div className="truncate text-xs text-slate-400">{session.login}</div>
        <div className="mt-0.5 inline-flex items-center gap-1 rounded bg-brand-50 px-1.5 py-0.5 text-[11px] text-brand-700">
          <ShieldCheck className="h-3 w-3" />{t("admin.verifiedBy")}
        </div>
      </div>
    </div>
  );
}

function Denied({ reason, session }: { reason: "forbidden" | "unavailable" | "error"; session: AdminSession | null }) {
  const { t } = useLanguage();
  // Without an identity the console was reached around `tailscale serve` (an SSH tunnel to
  // localhost), so point at the MagicDNS name instead.
  const url = `http://<MagicDNS>:${window.location.port || "80"}/admin`;
  let message: ReactNode = t("admin.loadError");
  if (reason === "unavailable") message = t("admin.unavailable");
  if (reason === "forbidden") {
    message = session?.login ? fill(t("admin.forbidden.notAllowed"), { login: session.login }) : fill(t("admin.forbidden.noTailscale"), { url });
  }
  return (
    <div className="flex h-full items-center justify-center bg-canvas p-6">
      <div className="max-w-md rounded-2xl border border-slate-200 bg-white p-6 text-center shadow-[0_4px_16px_rgba(15,23,42,0.06)]">
        <ShieldAlert className="mx-auto h-8 w-8 text-amber-500" />
        <h1 className="mt-3 text-lg font-semibold text-slate-900">{t("admin.forbidden")}</h1>
        <p className="mt-2 text-sm leading-relaxed text-slate-600">{message}</p>
        <a href="/" className="mt-5 inline-flex items-center gap-1.5 text-sm font-medium text-brand-700"><ArrowLeft className="h-4 w-4" />{t("admin.backToSite")}</a>
      </div>
    </div>
  );
}
