import { Download } from "lucide-react";
import { IS_DEMO } from "../lib/demo";
import { useLanguage } from "../lib/i18n";

/** Page-local export retains the exact filtered URL supplied by App. */
export default function ExportButton({ url }: { url: string | null }) {
  const { t } = useLanguage();
  if (IS_DEMO) return null; // a static site cannot export
  const className = "inline-flex shrink-0 items-center justify-center gap-1.5 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-700 hover:bg-slate-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500";
  return url ? <a href={url} download className={className}><Download aria-hidden="true" className="h-4 w-4" />{t("nav.export")}</a>
    : <button type="button" disabled title={t("nav.exportSoon")} className={`${className} cursor-not-allowed opacity-50`}><Download aria-hidden="true" className="h-4 w-4" />{t("nav.export")}</button>;
}
