import { Calculator, ExternalLink } from "lucide-react";
import { DEMO_REPOSITORY_URL } from "../../lib/demo";
import { useLanguage } from "../../lib/i18n";

/** Stands in for the estimate page, which needs the API, in the online demo. */
export default function DemoUnavailable({ onAnalysis }: { onAnalysis: () => void }) {
  const { t } = useLanguage();
  return <div className="flex h-full items-center justify-center overflow-y-auto p-4">
    <section className="w-full max-w-md rounded-2xl border border-slate-200 bg-white p-6 text-center shadow-sm sm:p-8">
      <span className="mx-auto flex h-14 w-14 items-center justify-center rounded-full bg-brand-50 text-brand-700"><Calculator aria-hidden="true" className="h-6 w-6" /></span>
      <h1 className="mt-4 text-lg font-semibold text-slate-900">{t("demo.unavailable.title")}</h1>
      <p className="mt-2 text-sm leading-relaxed text-slate-600">{t("demo.unavailable.body")}</p>
      {DEMO_REPOSITORY_URL && <a href={`${DEMO_REPOSITORY_URL}#readme`}
        className="mt-6 flex w-full items-center justify-center gap-2 rounded-xl bg-brand-600 px-4 py-2.5 text-sm font-semibold text-white hover:bg-brand-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 focus-visible:ring-offset-2">
        {t("demo.unavailable.deploy")}<ExternalLink aria-hidden="true" className="h-4 w-4" />
      </a>}
      <button type="button" onClick={onAnalysis} className="mt-4 rounded text-sm font-medium text-brand-700 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500">{t("demo.unavailable.analysis")}</button>
    </section>
  </div>;
}
