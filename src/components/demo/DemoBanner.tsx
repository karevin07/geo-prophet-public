import { useEffect, useState } from "react";
import { Github, X } from "lucide-react";
import { readSnapshotMeta } from "../../lib/api/snapshot-meta";
import { DEMO_REPOSITORY_URL } from "../../lib/demo";
import { useLanguage } from "../../lib/i18n";
import { fill } from "../../lib/utils";

const DISMISSED_KEY = "geoprophet.demoNoticeHidden";
const focusStyle = "rounded focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500";

function dismissed(): boolean {
  try {
    return localStorage.getItem(DISMISSED_KEY) === "1";
  } catch {
    return false;
  }
}

/** Says the site is the online demo, how recent its snapshot is and where the full app is. */
export default function DemoBanner() {
  const { t } = useLanguage();
  const [hidden, setHidden] = useState(dismissed);
  const [month, setMonth] = useState("…");

  useEffect(() => {
    if (hidden) return;
    const controller = new AbortController();
    readSnapshotMeta(`${import.meta.env.BASE_URL}demo-data/`, controller.signal)
      .then((meta) => setMonth(meta.through.slice(0, 7))).catch(() => undefined);
    return () => controller.abort();
  }, [hidden]);

  if (hidden) return null;
  const hide = () => {
    setHidden(true);
    try {
      localStorage.setItem(DISMISSED_KEY, "1");
    } catch {
      // Hiding it for this visit only is fine.
    }
  };
  return <div role="note" className="flex h-8 shrink-0 items-center gap-2 border-b border-brand-100 bg-brand-50 px-3 text-xs text-slate-700 sm:h-9 sm:gap-3 sm:px-6">
    <span className="shrink-0 rounded-full bg-brand-600 px-2 py-0.5 text-[11px] font-semibold text-white">{t("demo.badge")}</span>
    <span className="min-w-0 flex-1 truncate">
      <span className="sm:hidden">{fill(t("demo.summaryShort"), { month })}</span>
      <span className="hidden sm:inline">{fill(t("demo.summary"), { month })}</span>
    </span>
    {DEMO_REPOSITORY_URL && <>
      <a href={`${DEMO_REPOSITORY_URL}#readme`} className={`hidden shrink-0 font-medium text-brand-700 hover:underline md:inline ${focusStyle}`}>{t("demo.deploy")}</a>
      <a href={DEMO_REPOSITORY_URL} className={`hidden shrink-0 items-center gap-1 text-slate-700 hover:text-slate-900 sm:flex ${focusStyle}`}>
        <Github aria-hidden="true" className="h-3.5 w-3.5" />{t("demo.source")}
      </a>
    </>}
    <button type="button" onClick={hide} aria-label={t("demo.close")} title={t("demo.close")}
      className={`shrink-0 p-1 text-slate-400 hover:text-slate-600 ${focusStyle}`}><X aria-hidden="true" className="h-3.5 w-3.5" /></button>
  </div>;
}
