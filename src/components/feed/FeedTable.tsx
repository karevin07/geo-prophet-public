import { useLanguage } from "../../lib/i18n";
import { formatWan, priceClass, PRICE_CLASSES, type PriceBreaks } from "../../lib/price-scale";
import { cn, isoDate } from "../../lib/utils";
import { buildingCategory, type TransactionData } from "../../lib/api";

interface FeedTableProps {
  rows: TransactionData[];
  breaks: PriceBreaks | null;
  selectedId: string | null;
  onSelect: (transaction: TransactionData) => void;
}

export function floorText(transaction: TransactionData) {
  return `${transaction.floorLabel || "-"}${transaction.totalFloors ? ` / ${transaction.totalFloors}` : ""}`;
}

export function layoutText(transaction: TransactionData, isZh: boolean) {
  return isZh
    ? `${transaction.rooms}房${transaction.halls}廳${transaction.baths}衛`
    : `${transaction.rooms}BR ${transaction.halls}LR ${transaction.baths}BA`;
}

/** Parking type isn't in the list payload, so show the parking price when there is one. */
function parkingText(transaction: TransactionData, none: string) {
  return transaction.parkingPrice > 0 ? `${formatWan(transaction.parkingPrice, 0)} 萬` : none;
}

export function Badges({ transaction }: { transaction: TransactionData }) {
  const { t } = useLanguage();
  return (
    <>
      {transaction.isNew && <span className="shrink-0 rounded bg-brand-50 px-1.5 py-0.5 text-[10px] font-medium text-brand-700">{t("feed.new")}</span>}
      {transaction.specialReasons?.map((reason) => (
        <span key={reason} className="shrink-0 rounded bg-slate-100 px-1.5 py-0.5 text-[10px] font-medium text-slate-500">{t(`special.${reason}`)}</span>
      ))}
    </>
  );
}

export function PriceDot({ value, breaks }: { value: number; breaks: PriceBreaks | null }) {
  return <span className="inline-block h-2 w-2 shrink-0 rounded-full" style={{ backgroundColor: PRICE_CLASSES[priceClass(value, breaks)].hex }} />;
}

const COLUMN_WIDTHS = ["w-24", "", "w-20", "w-28", "w-24", "w-20", "w-20", "w-24", "w-16"];

/** Every address starts with its city; the table drops it to leave room for the street. */
function shortAddress(address: string) {
  return address.replace(/^.{2}[市縣]/, "");
}

export default function FeedTable({ rows, breaks, selectedId, onSelect }: FeedTableProps) {
  const { t, language } = useLanguage();
  const isZh = language === "zh";

  return (
    <>
      {/* Desktop table */}
      <div className="hidden overflow-x-auto md:block">
        <table className="w-full min-w-[900px] table-fixed text-sm">
          {/* Fixed widths let the address take the remaining space and truncate. */}
          <colgroup>
            {COLUMN_WIDTHS.map((width, index) => <col key={index} className={width} />)}
          </colgroup>
          <thead className="sticky top-0 z-10 bg-slate-50 text-xs text-slate-500">
            <tr className="border-b border-slate-200">
              {["date", "address", "type", "floor", "layout", "area", "total", "unit", "parking"].map((column) => (
                <th key={column} className={cn("whitespace-nowrap px-3 py-2.5 font-medium", ["area", "total", "unit"].includes(column) ? "text-right" : "text-left")}>
                  {t(`feed.col.${column}`)}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.map((transaction) => (
              <tr
                key={transaction.id}
                onClick={() => onSelect(transaction)}
                className={cn(
                  "cursor-pointer border-b border-slate-100 transition hover:bg-brand-50/60",
                  selectedId === transaction.id && "bg-brand-50 shadow-[inset_3px_0_0_#1f6f78]",
                )}
              >
                <td className="whitespace-nowrap px-3 py-3 tabular-nums text-slate-600">{isoDate(transaction.transactedAt)}</td>
                <td className="px-3 py-3">
                  <div className="flex items-center gap-1.5">
                    <span className="min-w-0 truncate text-slate-800" title={transaction.address}>{shortAddress(transaction.address)}</span>
                    <Badges transaction={transaction} />
                  </div>
                </td>
                <td className="whitespace-nowrap px-3 py-3 text-slate-600">{t(`type.${buildingCategory(transaction.buildingType)}`)}</td>
                <td className="truncate whitespace-nowrap px-3 py-3 text-slate-600" title={floorText(transaction)}>{floorText(transaction)}</td>
                <td className="whitespace-nowrap px-3 py-3 text-slate-600">{layoutText(transaction, isZh)}</td>
                <td className="whitespace-nowrap px-3 py-3 text-right tabular-nums text-slate-600">{transaction.buildingAreaPing.toFixed(1)}</td>
                <td className="whitespace-nowrap px-3 py-3 text-right font-medium tabular-nums text-slate-900">{formatWan(transaction.totalPrice, 0)}</td>
                <td className="whitespace-nowrap px-3 py-3 text-right tabular-nums">
                  <span className="inline-flex items-center gap-1.5 font-medium text-slate-900">
                    <PriceDot value={transaction.pricePerPing} breaks={breaks} />
                    {formatWan(transaction.pricePerPing)}
                  </span>
                </td>
                <td className="whitespace-nowrap px-3 py-3 text-slate-500">{parkingText(transaction, t("feed.none"))}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Mobile cards */}
      <ul className="divide-y divide-slate-100 md:hidden">
        {rows.map((transaction) => (
          <li key={transaction.id}>
            <button onClick={() => onSelect(transaction)} className="w-full px-4 py-3 text-left active:bg-brand-50">
              <div className="flex items-start justify-between gap-3">
                <span className="text-sm leading-snug text-slate-800">{transaction.address}</span>
                <span className="flex shrink-0 items-center gap-1.5 text-sm font-semibold tabular-nums text-slate-900">
                  <PriceDot value={transaction.pricePerPing} breaks={breaks} />
                  {formatWan(transaction.pricePerPing)}
                  <span className="text-[11px] font-normal text-slate-400">萬/坪</span>
                </span>
              </div>
              <div className="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-slate-500">
                <span className="tabular-nums">{isoDate(transaction.transactedAt)}</span>
                <span>{t(`type.${buildingCategory(transaction.buildingType)}`)}</span>
                <span>{transaction.buildingAreaPing.toFixed(1)} 坪</span>
                <span className="tabular-nums">{formatWan(transaction.totalPrice, 0)} 萬</span>
                <Badges transaction={transaction} />
              </div>
            </button>
          </li>
        ))}
      </ul>
    </>
  );
}
