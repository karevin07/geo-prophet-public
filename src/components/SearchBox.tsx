import { useEffect, useRef, useState } from "react";
import { Building2, Calculator, Loader2, MapPin, Route, Search, TrainFront, X } from "lucide-react";
import { useLanguage } from "../lib/i18n";
import type { Place, TransactionAPI } from "../lib/api";

interface SearchBoxProps {
  api: TransactionAPI;
  onSelect: (place: Place) => void;
  /** Only these kinds of places are offered; every kind by default. */
  kinds?: Place["kind"][];
  initialText?: string;
  placeholder?: string;
  /** Adds a button to value an address, next to address results. */
  onEstimate?: (place: Place) => void;
}

const icons = { district: Building2, road: Route, address: MapPin, station: TrainFront };

export default function SearchBox({ api, onSelect, kinds, initialText = "", placeholder, onEstimate }: SearchBoxProps) {
  const { t } = useLanguage();
  const [text, setText] = useState(initialText);
  // Typing opens results; restoring a chosen place's label does not.
  const typedRef = useRef(false);
  const kindKey = kinds?.join();
  const [results, setResults] = useState<Place[] | null>(null);
  const [active, setActive] = useState(0);
  const [isLoading, setIsLoading] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const query = text.trim();
    if (!typedRef.current || [...query].length < 2) {
      setResults(null);
      return;
    }
    const controller = new AbortController();
    const timer = window.setTimeout(async () => {
      setIsLoading(true);
      try {
        const places = await api.searchPlaces(query, controller.signal);
        setResults(kindKey ? places.filter((place) => kindKey.split(",").includes(place.kind)) : places);
        setActive(0);
      } catch {
        if (!controller.signal.aborted) setResults([]);
      } finally {
        if (!controller.signal.aborted) setIsLoading(false);
      }
    }, 250);
    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [api, text, kindKey]);

  useEffect(() => {
    const close = (event: MouseEvent) => {
      if (!containerRef.current?.contains(event.target as Node)) setResults(null);
    };
    document.addEventListener("mousedown", close);
    return () => document.removeEventListener("mousedown", close);
  }, []);

  const choose = (place: Place) => {
    onSelect(place);
    typedRef.current = false;
    setText(place.label);
    setResults(null);
  };

  return (
    <div ref={containerRef} className="relative">
      <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
      <input
        value={text}
        onChange={(event) => {
          typedRef.current = true;
          setText(event.target.value);
        }}
        onKeyDown={(event) => {
          if (!results?.length) return;
          if (event.key === "ArrowDown") setActive((index) => Math.min(index + 1, results.length - 1));
          if (event.key === "ArrowUp") setActive((index) => Math.max(index - 1, 0));
          if (event.key === "Enter") choose(results[active]);
          if (event.key === "Escape") setResults(null);
        }}
        placeholder={placeholder ?? t("search.placeholder")}
        aria-label={placeholder ?? t("search.placeholder")}
        className="h-10 w-full rounded-lg border border-slate-200 bg-slate-50 pl-9 pr-9 text-sm text-slate-800 outline-none transition placeholder:text-slate-400 focus:border-brand-500 focus:bg-white focus:ring-2 focus:ring-brand-100"
      />
      {isLoading ? (
        <Loader2 className="absolute right-3 top-1/2 h-4 w-4 -translate-y-1/2 animate-spin text-slate-400" />
      ) : text && (
        <button onClick={() => setText("")} aria-label="clear" className="absolute right-2 top-1/2 -translate-y-1/2 rounded p-1 text-slate-400 hover:text-slate-600">
          <X className="h-4 w-4" />
        </button>
      )}
      {results && (
        <ul className="absolute z-20 mt-1 max-h-72 w-full overflow-y-auto rounded-lg border border-slate-200 bg-white py-1 shadow-lg">
          {results.length === 0 && <li className="px-3 py-2 text-sm text-slate-400">{t("search.empty")}</li>}
          {results.map((place, index) => {
            const Icon = icons[place.kind];
            return (
              <li key={`${place.kind}-${place.label}-${index}`} className="flex items-center">
                <button
                  onMouseEnter={() => setActive(index)}
                  onClick={() => choose(place)}
                  className={`flex min-w-0 flex-1 items-center gap-2 px-3 py-2 text-left text-sm ${index === active ? "bg-brand-50 text-brand-700" : "text-slate-700"}`}
                >
                  <Icon className="h-4 w-4 shrink-0 text-slate-400" />
                  <span className="flex-1 truncate">{place.label}</span>
                  <span className="text-[11px] text-slate-400">{t(`search.${place.kind}`)}</span>
                </button>
                {onEstimate && place.kind === "address" && (
                  <button
                    onClick={() => {
                      setResults(null);
                      onEstimate(place);
                    }}
                    className="mr-2 flex shrink-0 items-center gap-1 rounded-md border border-brand-200 px-2 py-1 text-[11px] font-medium text-brand-700 hover:bg-brand-50"
                  >
                    <Calculator className="h-3 w-3" />{t("estimate.searchAction")}
                  </button>
                )}
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
