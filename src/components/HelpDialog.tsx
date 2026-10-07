import { useEffect } from "react";
import { X } from "lucide-react";
import { useLanguage } from "../lib/i18n";

export default function HelpDialog({ onClose }: { onClose: () => void }) {
  const { t } = useLanguage();
  useEffect(() => {
    const close = (event: KeyboardEvent) => event.key === "Escape" && onClose();
    window.addEventListener("keydown", close);
    return () => window.removeEventListener("keydown", close);
  }, [onClose]);

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/30 p-4" onClick={onClose}>
      <div role="dialog" aria-modal="true" aria-labelledby="help-title" className="w-full max-w-lg rounded-2xl bg-white p-6 shadow-xl" onClick={(event) => event.stopPropagation()}>
        <div className="mb-4 flex items-center justify-between">
          <h2 id="help-title" className="text-base font-semibold text-slate-900">{t("help.title")}</h2>
          <button onClick={onClose} aria-label={t("help.close")} className="rounded-md p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-600">
            <X className="h-4 w-4" />
          </button>
        </div>
        <ul className="list-disc space-y-2.5 pl-5 text-sm leading-relaxed text-slate-600">
          {["help.source", "help.price", "help.special", "help.yoy", "help.lag"].map((key) => <li key={key}>{t(key)}</li>)}
        </ul>
      </div>
    </div>
  );
}
