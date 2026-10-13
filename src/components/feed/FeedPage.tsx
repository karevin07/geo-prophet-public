import { useCallback, useEffect, useMemo, useState } from "react";
import { ChevronDown, Loader2 } from "lucide-react";
import { useLanguage } from "../../lib/i18n";
import { priceBreaks } from "../../lib/price-scale";
import { fill } from "../../lib/utils";
import { FEED_SORTS, type FeedQuery, type FeedSort, type Region, type Release, type TransactionAPI, type TransactionData } from "../../lib/api";
import FeedFilters from "./FeedFilters";
import FeedTable from "./FeedTable";
import TransactionDrawer from "./TransactionDrawer";
import ExportButton from "../ExportButton";

type FiltersProps = Omit<Parameters<typeof FeedFilters>[0], "regions" | "latestRelease">;

interface FeedPageProps extends FiltersProps {
  exportUrl?: string | null;
  api: TransactionAPI;
  query: FeedQuery;
  onShowOnMap: (transaction: TransactionData) => void;
  /** Opens a sale's community; the online demo has no communities. */
  onShowCommunity?: (id: string) => void;
}

interface FeedResult {
  rows: TransactionData[];
  total: number;
  newCount: number;
  nextCursor?: string;
  latestRelease: Release | null;
}

export default function FeedPage({ api, query, onShowOnMap, onShowCommunity, exportUrl, ...filters }: FeedPageProps) {
  const { t } = useLanguage();
  const [regions, setRegions] = useState<Region[]>([]);
  const [result, setResult] = useState<FeedResult | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isLoadingMore, setIsLoadingMore] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [selected, setSelected] = useState<TransactionData | null>(null);

  useEffect(() => {
    const controller = new AbortController();
    api.regions(controller.signal).then(setRegions).catch(() => undefined);
    return () => controller.abort();
  }, [api]);

  // Refetch the first page whenever the query changes (debounced like the map).
  useEffect(() => {
    const controller = new AbortController();
    const timer = window.setTimeout(async () => {
      setIsLoading(true);
      setError(null);
      try {
        const page = await api.feed(query, undefined, controller.signal);
        setResult({
          rows: page.data,
          total: page.meta.total ?? page.data.length,
          newCount: page.meta.newCount ?? 0,
          nextCursor: page.meta.nextCursor,
          latestRelease: page.meta.latestRelease,
        });
      } catch (requestError) {
        if (!controller.signal.aborted) setError(requestError instanceof Error ? requestError.message : "Unable to load data");
      } finally {
        if (!controller.signal.aborted) setIsLoading(false);
      }
    }, 250);
    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [api, query]);

  const loadMore = useCallback(async () => {
    if (!result?.nextCursor) return;
    setIsLoadingMore(true);
    try {
      const page = await api.feed(query, result.nextCursor);
      setResult((current) => current && { ...current, rows: [...current.rows, ...page.data], nextCursor: page.meta.nextCursor });
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : "Unable to load data");
    } finally {
      setIsLoadingMore(false);
    }
  }, [api, query, result?.nextCursor]);

  const breaks = useMemo(() => priceBreaks(result?.rows.map((row) => row.pricePerPing) ?? []), [result?.rows]);

  return (
    <div className="h-full overflow-y-auto bg-canvas">
      <div className="mx-auto flex max-w-[1360px] flex-col gap-5 p-4 sm:p-6 lg:flex-row lg:items-start">
        <div className="lg:sticky lg:top-0 lg:w-72 lg:shrink-0">
          <FeedFilters regions={regions} latestRelease={result?.latestRelease ?? null} {...filters} />
        </div>

        <section className="min-w-0 flex-1">
          <div className="mb-4 flex flex-wrap items-end justify-between gap-3">
            <div>
              <h1 className="text-xl font-semibold text-slate-900">{t("feed.title")}</h1>
              <p className="mt-1 text-sm text-slate-500">
                {result ? fill(t("feed.summary"), { total: result.total.toLocaleString(), fresh: result.newCount.toLocaleString() }) : t("status.loading")}
              </p>
            </div>
            <div className="flex flex-wrap items-center gap-3">
              <ExportButton url={exportUrl ?? null} />
              <label className="flex items-center gap-2 text-sm text-slate-600">
                {t("feed.sort")}
                <span className="relative">
                  <select
                    value={filters.feed.sort}
                    onChange={(event) => filters.setFeed({ sort: event.target.value as FeedSort })}
                    className="h-9 appearance-none rounded-lg border border-slate-200 bg-white pl-3 pr-8 text-sm text-slate-800 outline-none focus:border-brand-500 focus:ring-2 focus:ring-brand-100"
                  >
                    {FEED_SORTS.map((sort) => <option key={sort} value={sort}>{t(`feed.sort.${sort}`)}</option>)}
                  </select>
                  <ChevronDown className="pointer-events-none absolute right-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
                </span>
              </label>
            </div>
          </div>

          <div className="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-[0_4px_16px_rgba(15,23,42,0.06)]">
            {error && <div className="border-b border-red-100 bg-red-50 px-4 py-2 text-sm text-red-700">{error}</div>}
            {isLoading && !result ? (
              <div className="flex h-64 items-center justify-center text-sm text-slate-400"><Loader2 className="mr-2 h-4 w-4 animate-spin" />{t("status.loading")}</div>
            ) : result && result.rows.length === 0 ? (
              <div className="flex h-64 items-center justify-center text-sm text-slate-400">{t("feed.empty")}</div>
            ) : result && (
              <div className={isLoading ? "opacity-60 transition-opacity" : undefined}>
                <FeedTable rows={result.rows} breaks={breaks} selectedId={selected?.id ?? null} onSelect={setSelected} />
              </div>
            )}
          </div>

          {result?.nextCursor && (
            <div className="mt-4 flex justify-center">
              <button
                onClick={loadMore}
                disabled={isLoadingMore}
                className="flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-5 py-2 text-sm font-medium text-slate-700 hover:border-slate-300 disabled:opacity-60"
              >
                {isLoadingMore ? <Loader2 className="h-4 w-4 animate-spin" /> : <ChevronDown className="h-4 w-4" />}
                {fill(t("feed.loadMore"), { shown: result.rows.length.toLocaleString(), total: result.total.toLocaleString() })}
              </button>
            </div>
          )}
        </section>
      </div>

      {selected && (
        <TransactionDrawer transaction={selected} breaks={breaks} onClose={() => setSelected(null)} onShowOnMap={onShowOnMap} onShowCommunity={onShowCommunity} />
      )}
    </div>
  );
}
