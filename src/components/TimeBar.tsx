import { useEffect } from "react";
import { Lock, Pause, Play } from "lucide-react";
import { cn } from "../lib/utils";
import { useLanguage } from "../lib/i18n";

interface TimeBarProps {
  months: string[];
  /** Selected month, or null for the whole window. */
  month: string | null;
  setMonth: (month: string | null) => void;
  isPlaying: boolean;
  setIsPlaying: (playing: boolean) => void;
  /** Why the map ignores the time bar, such as the rent layer's fixed window; disables it. */
  fixedNote?: string | null;
}

const STEP_MS = 1500;
const label = (month: string) => month.replace("-", "/");

export default function TimeBar({ months, month, setMonth, isPlaying, setIsPlaying, fixedNote }: TimeBarProps) {
  const { t } = useLanguage();
  const index = month ? months.indexOf(month) : months.length - 1;

  // Advance one month per step and stop on the last month.
  useEffect(() => {
    if (fixedNote) setIsPlaying(false);
  }, [fixedNote, setIsPlaying]);

  useEffect(() => {
    if (!isPlaying) return;
    const timer = window.setTimeout(() => {
      if (index >= months.length - 1) setIsPlaying(false);
      else setMonth(months[index + 1]);
    }, STEP_MS);
    return () => window.clearTimeout(timer);
  }, [isPlaying, index, months, setMonth, setIsPlaying]);

  const togglePlay = () => {
    if (isPlaying) {
      setIsPlaying(false);
      return;
    }
    // Start from the first month unless a month in the middle is already selected.
    if (!month || index >= months.length - 1) setMonth(months[0]);
    setIsPlaying(true);
  };

  return (
    <div className="absolute bottom-3 left-3 right-3 z-20 flex items-center gap-3 rounded-full border border-slate-200/80 bg-white/95 py-2 pl-2 pr-4 shadow-[0_4px_16px_rgba(15,23,42,0.08)] backdrop-blur sm:bottom-4 sm:left-1/2 sm:right-auto sm:w-[34rem] sm:-translate-x-1/2 lg:ml-40">
      <button
        onClick={togglePlay}
        disabled={Boolean(fixedNote)}
        aria-label={isPlaying ? t("time.pause") : t("time.play")}
        className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-brand-600 text-white hover:bg-brand-700 disabled:bg-slate-200 disabled:text-slate-400"
      >
        {isPlaying ? <Pause className="h-4 w-4" /> : <Play className="ml-0.5 h-4 w-4" />}
      </button>
      <div className={cn("min-w-0 flex-1", fixedNote && "opacity-50")}>
        <div className="flex justify-between text-[10px] tabular-nums text-slate-400">
          <span>{label(months[0])}</span>
          <span className={cn("rounded px-1.5 font-medium", month ? "bg-brand-50 text-brand-700" : "text-slate-400")}>
            {month ? label(month) : t("time.all")}
          </span>
          <span>{label(months[months.length - 1])}</span>
        </div>
        <input
          type="range" min={0} max={months.length - 1} step={1} value={index} disabled={Boolean(fixedNote)}
          onChange={(event) => {
            setIsPlaying(false);
            setMonth(months[Number(event.target.value)]);
          }}
          aria-label={month ? label(month) : t("time.all")}
          className={cn("w-full", !month && "opacity-60")}
        />
      </div>
      {fixedNote ? (
        <span className="flex shrink-0 items-center gap-1 rounded-full border border-brand-100 bg-brand-50 px-3 py-1 text-xs font-medium text-brand-700">
          <Lock className="h-3 w-3" />{fixedNote}
        </span>
      ) : <button
        onClick={() => {
          setIsPlaying(false);
          setMonth(null);
        }}
        aria-pressed={!month}
        className={cn("shrink-0 rounded-full border px-3 py-1 text-xs font-medium", month ? "border-slate-200 text-slate-600 hover:border-slate-300" : "border-brand-100 bg-brand-50 text-brand-700")}
      >
        {t("time.all")}
      </button>}
    </div>
  );
}
