import { useEffect, useRef, useState, type MouseEvent } from "react";
import { Activity, Building2, ChartNoAxesCombined, Check, ChevronDown, CircleDollarSign, HelpCircle, Map as MapIcon, MapPin, Menu, Scale, ShieldCheck, TrainFront, X, type LucideIcon } from "lucide-react";
import { cn } from "../lib/utils";
import { useLanguage } from "../lib/i18n";
import { NAVIGATION_GROUPS, navigationGroup, navigationHref, type NavigationGroup } from "../lib/navigation";
import type { Tab } from "../lib/url-state";

interface TopBarProps {
  tab: Tab;
  setTab: (tab: Tab) => void;
  onHelp: () => void;
  /** Kept for callers; export now lives in the page toolbar. */
  exportUrl: string | null;
  showAdmin: boolean;
}

const ICONS: Record<Tab, LucideIcon> = {
  map: MapIcon, feed: Activity, analysis: ChartNoAxesCombined, rents: CircleDollarSign,
  presales: Building2, area: MapPin, districts: Scale, stations: TrainFront,
  investment: ChartNoAxesCombined, estimate: CircleDollarSign,
};
const focusStyle = "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 focus-visible:ring-offset-2";

function LanguageSwitch() {
  const { t, language, setLanguage } = useLanguage();
  return <div role="group" aria-label={t("nav.language")} className="flex shrink-0 rounded-full bg-slate-100 p-0.5 text-xs font-medium">
    {(["zh", "en"] as const).map((code) => <button key={code} type="button" aria-pressed={language === code}
      onClick={() => setLanguage(code)} className={cn("rounded-full px-2.5 py-1", focusStyle, language === code ? "bg-white text-brand-700 shadow-sm" : "text-slate-500")}>
      {code === "zh" ? "中" : "EN"}
    </button>)}
  </div>;
}

export default function TopBar({ tab, setTab, onHelp, showAdmin }: TopBarProps) {
  const { t } = useLanguage();
  const [openGroup, setOpenGroup] = useState<NavigationGroup | null>(null);
  const navRef = useRef<HTMLElement>(null);
  const dialogRef = useRef<HTMLDialogElement>(null);
  const drawerButton = useRef<HTMLButtonElement>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const activeGroup = navigationGroup(tab);

  useEffect(() => {
    setOpenGroup(null);
    dialogRef.current?.close();
  }, [tab]);

  useEffect(() => {
    if (!openGroup) return;
    const dismiss = (event: PointerEvent | FocusEvent | KeyboardEvent) => {
      if (event instanceof KeyboardEvent) {
        if (event.key !== "Escape") return;
        navRef.current?.querySelector<HTMLButtonElement>(`[data-group="${openGroup}"]`)?.focus();
      } else if (navRef.current?.contains(event.target as Node)) return;
      setOpenGroup(null);
    };
    document.addEventListener("pointerdown", dismiss);
    document.addEventListener("focusin", dismiss);
    document.addEventListener("keydown", dismiss);
    return () => {
      document.removeEventListener("pointerdown", dismiss);
      document.removeEventListener("focusin", dismiss);
      document.removeEventListener("keydown", dismiss);
    };
  }, [openGroup]);

  useEffect(() => {
    const desktop = window.matchMedia("(min-width: 1024px)");
    const close = () => { setOpenGroup(null); dialogRef.current?.close(); };
    desktop.addEventListener("change", close);
    return () => desktop.removeEventListener("change", close);
  }, []);

  const navigate = (event: MouseEvent<HTMLAnchorElement>, next: Tab) => {
    // Native links retain Cmd/Ctrl-click, new-tab and context-menu behaviour.
    if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    event.preventDefault();
    setOpenGroup(null);
    dialogRef.current?.close();
    setTab(next);
  };

  const pageLink = (item: Tab, description = false) => {
    const Icon = ICONS[item];
    return <a key={item} href={navigationHref(item)} onClick={(event) => navigate(event, item)}
      aria-current={tab === item ? "page" : undefined}
      className={cn("flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm", focusStyle,
        tab === item ? "bg-brand-50 font-semibold text-brand-700" : "text-slate-700 hover:bg-slate-50")}>
      <Icon aria-hidden="true" className="h-4 w-4 shrink-0" />
      <span className="min-w-0 flex-1">
        <span className="block">{t(item === "map" ? "nav.explore" : `nav.${item}`)}</span>
        {description && <span className="mt-0.5 block text-xs font-normal leading-relaxed text-slate-500">{t(`nav.description.${item}`)}</span>}
      </span>
      {tab === item && <Check aria-hidden="true" className="h-4 w-4 shrink-0" />}
    </a>;
  };

  return <header className="relative z-40 flex h-14 shrink-0 items-center gap-3 border-b border-slate-200 bg-white px-3 sm:h-16 sm:px-6 lg:gap-6">
    <a href="/" aria-label={t("app.title")} onClick={(event) => navigate(event, "map")} className={cn("flex shrink-0 items-center gap-2.5 rounded-lg", focusStyle)}>
      <span className="flex h-8 w-8 items-center justify-center rounded-lg bg-brand-600 text-white"><MapIcon aria-hidden="true" className="h-4.5 w-4.5" /></span>
      <span className="hidden text-base font-semibold text-slate-900 sm:inline">{t("app.title")}</span>
    </a>

    <div className="min-w-0 flex-1 lg:hidden">
      <p className="truncate text-sm font-semibold text-slate-900">{t(`nav.${tab}`)}</p>
      <p className="truncate text-[11px] text-slate-500">{t(activeGroup ? `nav.group.${activeGroup}` : "nav.explore")}</p>
    </div>

    <nav ref={navRef} aria-label={t("nav.main")} className="hidden min-w-0 flex-1 items-stretch gap-1 self-stretch lg:flex xl:gap-3">
      <a href="/" onClick={(event) => navigate(event, "map")} aria-current={tab === "map" ? "page" : undefined}
        className={cn("flex shrink-0 items-center gap-2 border-b-2 px-3 text-sm font-medium", focusStyle, tab === "map" ? "border-brand-600 text-brand-700" : "border-transparent text-slate-500 hover:text-slate-900")}>
        <MapIcon aria-hidden="true" className="h-4 w-4" />{t("nav.explore")}
      </a>
      {NAVIGATION_GROUPS.map((group) => <div key={group.id} className="relative flex" onBlur={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget)) setOpenGroup((current) => current === group.id ? null : current);
      }}>
        <button type="button" data-group={group.id} aria-expanded={openGroup === group.id} aria-controls={`nav-${group.id}`}
          onClick={() => setOpenGroup((current) => current === group.id ? null : group.id)}
          onKeyDown={(event) => {
            if (event.key === "ArrowDown") {
              event.preventDefault(); setOpenGroup(group.id);
              requestAnimationFrame(() => document.getElementById(`nav-${group.id}`)?.querySelector("a")?.focus());
            }
          }}
          className={cn("flex shrink-0 items-center gap-1.5 border-b-2 px-3 text-sm font-medium", focusStyle,
            activeGroup === group.id ? "border-brand-600 text-brand-700" : "border-transparent text-slate-500 hover:text-slate-900",
            openGroup === group.id && "bg-slate-50 text-brand-700")}>
          {t(`nav.group.${group.id}`)}<ChevronDown aria-hidden="true" className={cn("h-3.5 w-3.5 transition-transform", openGroup === group.id && "rotate-180")} />
        </button>
        <div id={`nav-${group.id}`} hidden={openGroup !== group.id} className="absolute left-0 top-full z-50 mt-2 w-80 rounded-2xl border border-slate-200 bg-white p-2 shadow-xl">
          {group.tabs.map((item) => pageLink(item, true))}
        </div>
      </div>)}
    </nav>

    <div className="flex shrink-0 items-center gap-2">
      <div className="hidden sm:block"><LanguageSwitch /></div>
      {showAdmin && <a href="/admin" aria-label={t("nav.admin")} title={t("nav.admin")}
        className={cn("flex h-9 w-9 items-center justify-center rounded-full text-slate-500 hover:bg-slate-50 hover:text-brand-700", focusStyle)}><ShieldCheck aria-hidden="true" className="h-4 w-4" /></a>}
      <button type="button" onClick={onHelp} aria-label={t("nav.help")} title={t("nav.help")}
        className={cn("hidden h-9 w-9 items-center justify-center rounded-full text-slate-500 hover:bg-slate-50 sm:flex", focusStyle)}><HelpCircle aria-hidden="true" className="h-4 w-4" /></button>
      <button ref={drawerButton} type="button" aria-label={t("nav.open")} aria-haspopup="dialog" aria-controls="navigation-drawer" aria-expanded={drawerOpen}
        onClick={() => { dialogRef.current?.showModal(); setDrawerOpen(true); }}
        className={cn("flex h-10 w-10 items-center justify-center rounded-lg text-slate-600 hover:bg-slate-50 lg:hidden", focusStyle)}><Menu aria-hidden="true" className="h-5 w-5" /></button>
    </div>

    <dialog ref={dialogRef} id="navigation-drawer" aria-labelledby="navigation-title"
      onClose={() => { setDrawerOpen(false); drawerButton.current?.focus(); }}
      onClick={(event) => {
        if (event.target === event.currentTarget) {
          const box = event.currentTarget.getBoundingClientRect();
          if (event.clientX < box.left || event.clientX > box.right || event.clientY < box.top || event.clientY > box.bottom) event.currentTarget.close();
        }
      }}
      className="fixed inset-y-0 left-auto right-0 m-0 h-dvh max-h-none w-[min(22rem,calc(100vw-2rem))] max-w-none border-0 bg-white p-0 text-slate-700 shadow-2xl backdrop:bg-slate-900/40">
      <div className="flex h-full flex-col">
        <div className="flex shrink-0 items-center justify-between border-b border-slate-100 px-5 py-4">
          <h2 id="navigation-title" className="font-semibold text-slate-900">{t("nav.main")}</h2>
          <button type="button" autoFocus aria-label={t("nav.close")} onClick={() => dialogRef.current?.close()} className={cn("rounded-lg p-2 hover:bg-slate-100", focusStyle)}><X aria-hidden="true" className="h-5 w-5" /></button>
        </div>
        <nav aria-label={t("nav.main")} className="min-h-0 flex-1 space-y-5 overflow-y-auto p-4">
          {pageLink("map")}
          {NAVIGATION_GROUPS.map((group) => <section key={group.id} aria-labelledby={`drawer-${group.id}`}>
            <h3 id={`drawer-${group.id}`} className="mb-1 px-3 text-xs font-semibold tracking-wide text-slate-500">{t(`nav.group.${group.id}`)}</h3>
            {group.tabs.map((item) => pageLink(item))}
          </section>)}
        </nav>
        <div className="flex shrink-0 items-center justify-between gap-3 border-t border-slate-100 p-4">
          <LanguageSwitch />
          <button type="button" onClick={() => { dialogRef.current?.close(); onHelp(); }} className={cn("flex items-center gap-2 rounded-lg p-2 text-sm text-slate-600 hover:bg-slate-50", focusStyle)}><HelpCircle aria-hidden="true" className="h-4 w-4" />{t("nav.help")}</button>
        </div>
      </div>
    </dialog>
  </header>;
}
