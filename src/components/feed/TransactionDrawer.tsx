import { useEffect, useState } from "react";
import { Building, Check, Copy, MapPin, X } from "lucide-react";
import { useLanguage } from "../../lib/i18n";
import { formatWan, priceClass, PRICE_CLASSES, type PriceBreaks } from "../../lib/price-scale";
import { fill, isoDate } from "../../lib/utils";
import type { TransactionData } from "../../lib/api";
import { Badges, floorText, layoutText } from "./FeedTable";

interface TransactionDrawerProps {
  transaction: TransactionData;
  breaks: PriceBreaks | null;
  onClose: () => void;
  onShowOnMap: (transaction: TransactionData) => void;
  onShowCommunity?: (id: string) => void;
}

function ageYears(transaction: TransactionData) {
  if (!transaction.completedAt) return null;
  const years = (Date.parse(transaction.transactedAt) - Date.parse(transaction.completedAt)) / (365.25 * 86_400_000);
  return years >= 0 ? Math.floor(years) : null;
}

export default function TransactionDrawer({ transaction, breaks, onClose, onShowOnMap, onShowCommunity }: TransactionDrawerProps) {
  const { t, language } = useLanguage();
  const [copied, setCopied] = useState(false);
  const age = ageYears(transaction);

  useEffect(() => {
    const close = (event: KeyboardEvent) => event.key === "Escape" && onClose();
    window.addEventListener("keydown", close);
    return () => window.removeEventListener("keydown", close);
  }, [onClose]);
  useEffect(() => setCopied(false), [transaction.id]);

  const details: [string, string][] = [
    [t("drawer.buildingType"), transaction.buildingType],
    [t("drawer.mainUse"), transaction.mainUse || t("feed.none")],
    [t("drawer.floor"), floorText(transaction)],
    [t("drawer.layout"), layoutText(transaction, language === "zh")],
    [t("drawer.area"), `${transaction.buildingAreaPing.toFixed(1)} 坪（${t("drawer.excludesParking")}）`],
    [t("drawer.parking"), transaction.parkingPrice > 0 ? `${formatWan(transaction.parkingPrice, 0)} 萬` : t("feed.none")],
    [t("drawer.age"), age != null && transaction.completedAt ? fill(t("drawer.ageValue"), { years: age, date: transaction.completedAt.slice(0, 7) }) : t("feed.none")],
    [t("drawer.transactionType"), transaction.transactionType],
    [t("drawer.precision"), t(`precision.${transaction.geocodePrecision}`)],
  ];

  return (
    <>
      <div className="fixed inset-0 top-14 z-30 bg-slate-900/10 sm:top-16" onClick={onClose} />
      <aside
        role="dialog"
        aria-modal="true"
        aria-label={transaction.address}
        className="fixed bottom-0 right-0 top-14 z-40 flex w-full max-w-md flex-col bg-white shadow-[-8px_0_24px_rgba(15,23,42,0.08)] sm:top-16"
      >
        <header className="border-b border-slate-100 px-5 pb-4 pt-5">
          <div className="flex items-start gap-2">
            <span className="mt-1.5 h-2.5 w-2.5 shrink-0 rounded-full" style={{ backgroundColor: PRICE_CLASSES[priceClass(transaction.pricePerPing, breaks)].hex }} />
            <h2 className="flex-1 text-base font-semibold leading-snug text-slate-900">{transaction.address}</h2>
            <button onClick={onClose} aria-label={t("drawer.close")} className="rounded-md p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-600">
              <X className="h-4 w-4" />
            </button>
          </div>
          <p className="mt-1.5 break-all pl-4.5 text-[11px] tabular-nums text-slate-500">
            {fill(t("drawer.meta"), { date: isoDate(transaction.transactedAt), season: transaction.season, id: transaction.id })}
          </p>
          <div className="mt-2 flex flex-wrap gap-1.5 pl-4.5"><Badges transaction={transaction} /></div>
        </header>

        <div className="flex-1 space-y-5 overflow-y-auto px-5 py-5">
          <div className="grid grid-cols-2 rounded-xl border border-brand-100 bg-brand-50 px-4 py-3">
            <div>
              <div className="text-xs text-slate-500">{t("drawer.total")}</div>
              <div className="mt-1 text-2xl font-semibold tabular-nums text-slate-900">{formatWan(transaction.totalPrice, 0)}<span className="ml-1 text-sm font-normal text-slate-500">萬</span></div>
            </div>
            <div className="border-l border-brand-100 pl-4 text-right">
              <div className="text-xs text-slate-500">{t("drawer.unit")}</div>
              <div className="mt-1 text-2xl font-semibold tabular-nums text-brand-700">{formatWan(transaction.pricePerPing)}<span className="ml-1 text-sm font-normal text-slate-500">萬/坪</span></div>
              <div className="text-[11px] text-slate-400">{t("drawer.excludesParking")}</div>
            </div>
          </div>

          <section>
            <h3 className="mb-2 text-xs font-semibold text-slate-500">{t("drawer.details")}</h3>
            <dl className="grid grid-cols-2 gap-x-4 gap-y-3 rounded-xl border border-slate-200 p-4 text-sm">
              {details.map(([label, value]) => (
                <div key={label} className="min-w-0">
                  <dt className="text-[11px] text-slate-400">{label}</dt>
                  <dd className="mt-0.5 break-words text-slate-800">{value}</dd>
                </div>
              ))}
            </dl>
          </section>

          {transaction.remarks && (
            <section>
              <h3 className="mb-2 text-xs font-semibold text-slate-500">{t("drawer.remarks")}</h3>
              <p className="rounded-xl border border-amber-100 bg-amber-50 px-4 py-3 text-sm leading-relaxed text-slate-700">{transaction.remarks}</p>
            </section>
          )}
        </div>

        <footer className="space-y-2 border-t border-slate-100 p-4">
          <button onClick={() => onShowOnMap(transaction)} className="flex w-full items-center justify-center gap-2 rounded-lg bg-brand-600 py-2.5 text-sm font-medium text-white hover:bg-brand-700">
            <MapPin className="h-4 w-4" />
            {t("drawer.showOnMap")}
          </button>
          {onShowCommunity && transaction.communityId && (
            <button onClick={() => onShowCommunity(transaction.communityId!)}
              className="flex w-full items-center justify-center gap-2 rounded-lg border border-brand-200 py-2.5 text-sm font-medium text-brand-700 hover:bg-brand-50">
              <Building className="h-4 w-4" />
              {t("community.open")}
            </button>
          )}
          <button
            onClick={async () => {
              try {
                await navigator.clipboard.writeText(transaction.id);
                setCopied(true);
              } catch {
                setCopied(false);
              }
            }}
            className="flex w-full items-center justify-center gap-1.5 py-1.5 text-xs text-slate-500 hover:text-slate-800"
          >
            {copied ? <Check className="h-3.5 w-3.5 text-green-600" /> : <Copy className="h-3.5 w-3.5" />}
            {copied ? t("drawer.copied") : `${t("drawer.copyId")} ${transaction.id}`}
          </button>
        </footer>
      </aside>
    </>
  );
}
