import { lazy, Suspense, useCallback, useEffect, useMemo, useState } from "react";
import FeedPage from "./components/feed/FeedPage";
import FilterPanel from "./components/FilterPanel";
import HelpDialog from "./components/HelpDialog";
import Legend from "./components/Legend";
import TimeBar from "./components/TimeBar";
import DemoBanner from "./components/demo/DemoBanner";
import TopBar from "./components/TopBar";
import type { FlyTarget } from "./components/Map";
import {
  BUILDING_TYPES, createTransactionAPI,
  type BBox, type BuildingType, type DistrictBoundaries, type FeedQuery, type TransitLines, type TransitStation, type H3Aggregate, type Place, type SummaryWithYoY, type TransactionData, type TransactionQuery, type TrendPoint,
} from "./lib/api";
import { adminAPI } from "./lib/api/admin";
import { IS_DEMO } from "./lib/demo";
import { useLanguage } from "./lib/i18n";
import { priceBreaks, recentMonths } from "./lib/price-scale";
import { loadLayerSettings, saveLayerSettings, type LayerSettings } from "./lib/layer-settings";
import { DEFAULT_STATE, formatAppState, parseAppState, type AppState, type FeedState, type InvestmentState, type RentsState, type EstimateState, type PresalesState, type AreaState, type MapView as MapCamera, type Tab } from "./lib/url-state";
import { sameBBox } from "./lib/viewport";

const transactionAPI = createTransactionAPI();
const MapView = lazy(() => import("./components/Map"));
const AnalysisPage = lazy(() => import("./components/analysis/AnalysisPage"));
const DistrictsPage = lazy(() => import("./components/districts/DistrictsPage"));
const StationsPage = lazy(() => import("./components/StationsPage"));
const InvestmentPage = lazy(() => import("./components/investment/InvestmentPage"));
const RentsPage = lazy(() => import("./components/rents/RentsPage"));
const EstimatePage = lazy(() => import("./components/estimate/EstimatePage"));
const DemoUnavailable = lazy(() => import("./components/demo/DemoUnavailable"));
const PresalesPage = lazy(() => import("./components/presales/PresalesPage"));
const AreaPage = lazy(() => import("./components/area/AreaPage"));

const PLACE_ZOOM = { district: 13, road: 15, address: 17 };

export default function App() {
  const { t } = useLanguage();
  const months = useMemo(() => recentMonths(), []);
  const [initial] = useState(() => parseAppState(window.location.pathname, window.location.search));

  const [tab, setTabState] = useState<Tab>(initial.tab);
  const [priceRange, setPriceRange] = useState(initial.priceRange);
  const [areaRange, setAreaRange] = useState(initial.areaRange);
  const [selectedTypes, setSelectedTypes] = useState<BuildingType[]>(initial.types);
  const [includeSpecial, setIncludeSpecial] = useState(initial.includeSpecial);
  const [month, setMonth] = useState<string | null>(initial.month && months.includes(initial.month) ? initial.month : null);
  const [viewMode, setViewMode] = useState(initial.viewMode);
  const [h3Resolution, setH3Resolution] = useState(initial.h3Resolution);
  const [camera, setCamera] = useState<MapCamera>(initial.view);
  const [feed, setFeedState] = useState<FeedState>(initial.feed);
  const [investment, setInvestmentState] = useState<InvestmentState>(initial.investment);
  const setFeed = useCallback((update: Partial<FeedState>) => setFeedState((current) => ({ ...current, ...update })), []);
  const setInvestment = useCallback((update: Partial<InvestmentState>) => setInvestmentState((current) => ({ ...current, ...update })), []);
  const [rents, setRentsState] = useState<RentsState>(initial.rents);
  const setRents = useCallback((update: Partial<RentsState>) => setRentsState((current) => ({ ...current, ...update })), []);
  const [presales, setPresalesState] = useState<PresalesState>(initial.presales);
  const setPresales = useCallback((update: Partial<PresalesState>) => setPresalesState((current) => ({ ...current, ...update })), []);
  const [area, setAreaState] = useState<AreaState>(initial.area);
  const setArea = useCallback((update: Partial<AreaState>) => setAreaState((current) => ({ ...current, ...update })), []);
  const [estimate, setEstimateState] = useState<EstimateState>(initial.estimate);
  const setEstimate = useCallback((update: Partial<EstimateState>) => setEstimateState((current) => ({ ...current, ...update })), []);

  const [transactions, setTransactions] = useState<TransactionData[]>([]);
  const [hexData, setHexData] = useState<H3Aggregate[]>([]);
  const [trends, setTrends] = useState<TrendPoint[]>([]);
  const [summary, setSummary] = useState<SummaryWithYoY | null>(null);
  const [isPlaying, setIsPlaying] = useState(false);
  const [bbox, setBBox] = useState<BBox | null>(null);
  const [isPanelOpen, setIsPanelOpen] = useState(() => window.innerWidth >= 640);
  const [isHelpOpen, setIsHelpOpen] = useState(false);
  // Only admins see the way into the admin console; anyone else gets no link.
  const [isAdmin, setIsAdmin] = useState(false);
  useEffect(() => {
    if (import.meta.env.VITE_DATA_SOURCE === "mock" || IS_DEMO) return;
    const controller = new AbortController();
    adminAPI.me(controller.signal).then((session) => setIsAdmin(session.admin)).catch(() => undefined);
    return () => controller.abort();
  }, []);
  const [flyTarget, setFlyTarget] = useState<FlyTarget | null>(null);
  // A shared station link needs the transit overlay, whatever this viewer last chose.
  const [layerSettings, setLayerSettings] = useState<LayerSettings>(() => {
    const settings = loadLayerSettings();
    return initial.station && !settings.transit ? { ...settings, transit: true } : settings;
  });
  const [districtBoundaries, setDistrictBoundaries] = useState<DistrictBoundaries | null>(null);
  const [transitLines, setTransitLines] = useState<TransitLines | null>(null);
  const [stations, setStations] = useState<TransitStation[]>([]);
  const [stationYoY, setStationYoY] = useState<Map<string, number | null>>(() => new Map());
  // A station to open on the map: from a shared link, the search box or the station ranking.
  const [focusStationId, setFocusStationId] = useState<string | null>(initial.station);
  // The station whose card is open, kept in the URL so the view can be shared.
  const [stationId, setStationId] = useState<string | null>(initial.station);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const appState = useMemo<AppState>(() => ({
    tab, priceRange, areaRange, types: selectedTypes, includeSpecial, month, viewMode, h3Resolution, view: camera, station: stationId, feed, investment, rents, estimate, presales, area,
  }), [tab, priceRange, areaRange, selectedTypes, includeSpecial, month, viewMode, h3Resolution, camera, stationId, feed, investment, rents, estimate, presales, area]);

  // Keep the URL in sync so links can be shared and reloads restore the view. Tabs push a
  // history entry (back/forward switch pages); everything else replaces the current one.
  useEffect(() => {
    const url = formatAppState(appState);
    if (url !== window.location.pathname + window.location.search) window.history.replaceState(null, "", url);
  }, [appState]);

  const setTab = useCallback((next: Tab) => {
    setTabState(next);
    window.history.pushState(null, "", formatAppState({ ...appState, tab: next }));
  }, [appState]);

  useEffect(() => {
    const restore = () => setTabState(parseAppState(window.location.pathname, window.location.search).tab);
    window.addEventListener("popstate", restore);
    return () => window.removeEventListener("popstate", restore);
  }, []);

  const query = useMemo<TransactionQuery | null>(() => bbox && ({
    priceRange, areaRange, types: selectedTypes, bbox, includeSpecial, month: month ?? undefined,
  }), [priceRange, areaRange, selectedTypes, bbox, includeSpecial, month]);

  // The map reports its visible area and camera after it loads and whenever it settles.
  const handleCameraChange = useCallback((next: BBox, view: MapCamera) => {
    setBBox((previous) => sameBBox(previous, next) ? previous : next);
    setCamera(view);
  }, []);

  const handleSelectPlace = useCallback((place: Place) => {
    if (place.kind === "station" && place.stationId) {
      // The map flies to the station itself, leaving room for its card.
      setFocusStationId(place.stationId);
      setLayerSettings((current) => current.transit ? current : { ...current, transit: true });
      return;
    }
    setFlyTarget({ lat: place.lat, lng: place.lng, zoom: PLACE_ZOOM[place.kind as keyof typeof PLACE_ZOOM] });
  }, []);

  // A new address starts a new estimate; the type stays as a starting guess.
  const estimatePlace = useCallback((place: Place) => {
    setEstimateState((current) => ({ ...DEFAULT_STATE.estimate, type: current.type, address: place.label, lat: place.lat, lng: place.lng }));
    setTab("estimate");
  }, [setTab]);

  const resetFilters = useCallback(() => {
    setPriceRange(DEFAULT_STATE.priceRange);
    setAreaRange(DEFAULT_STATE.areaRange);
    setSelectedTypes([...BUILDING_TYPES]);
    setIncludeSpecial(false);
  }, []);

  const resetFeed = useCallback(() => {
    resetFilters();
    setFeedState(DEFAULT_STATE.feed);
  }, [resetFilters]);

  // The feed shares the price, area, type and special filters with the map, adds its own
  // region and period, and is not limited to the map viewport.
  const feedQuery = useMemo<FeedQuery>(() => ({
    priceRange, areaRange, types: selectedTypes, includeSpecial,
    from: feed.period === 0 ? "2000-01" : months[Math.max(0, months.length - feed.period)],
    to: months[months.length - 1],
    city: feed.city || undefined,
    district: feed.district || undefined,
    latestOnly: feed.latestOnly,
    sort: feed.sort,
  }), [priceRange, areaRange, selectedTypes, includeSpecial, feed, months]);

  // Opens the map on one sale: point view at street level, in its month when it is recent.
  const showOnMap = useCallback((sale: TransactionData) => {
    const saleMonth = sale.transactedAt.slice(0, 7);
    setCamera({ lat: sale.lat, lng: sale.lng, zoom: 17 });
    setViewMode("points");
    setMonth(months.includes(saleMonth) ? saleMonth : null);
    setIsPanelOpen(window.innerWidth >= 640);
    setTab("map");
  }, [months, setTab]);

  const showPoint = useCallback((lat: number, lng: number) => {
    setCamera({ lat, lng, zoom: 16 });
    setIsPanelOpen(window.innerWidth >= 640);
    setTab("map");
  }, [setTab]);

  // Opens a presale project, e.g. from a station card.
  const showProject = useCallback((project: { key: string; cityCode: string }) => {
    setPresalesState((current) => ({ ...current, city: project.cityCode as PresalesState["city"], district: "", project: project.key }));
    setTab("presales");
  }, [setTab]);

  const showStation = useCallback((station: TransitStation) => {
    setCamera({ lat: station.lat, lng: station.lng, zoom: 15 });
    setFocusStationId(station.id);
    setLayerSettings((current) => current.transit ? current : { ...current, transit: true });
    setIsPanelOpen(window.innerWidth >= 640);
    setTab("map");
  }, [setTab]);

  const updateLayerSettings = useCallback((update: Partial<LayerSettings>) => {
    setLayerSettings((current) => {
      const next = { ...current, ...update };
      saveLayerSettings(next);
      return next;
    });
  }, []);

  // The district overlay covers every supported city; fetch them once, when first shown.
  useEffect(() => {
    if (!layerSettings.districts || districtBoundaries) return;
    const controller = new AbortController();
    transactionAPI.regions(controller.signal)
      .then((regions) => Promise.all(regions.map((region) => transactionAPI.districtBoundaries(region.code, controller.signal))))
      .then((collections) => setDistrictBoundaries({ type: "FeatureCollection", features: collections.flatMap((collection) => collection.features) }))
      .catch(() => undefined);
    return () => controller.abort();
  }, [layerSettings.districts, districtBoundaries]);

  // Rail lines and stations cover every city; fetch them once, when the overlay is first shown.
  useEffect(() => {
    if (!layerSettings.transit || transitLines) return;
    const controller = new AbortController();
    Promise.all([transactionAPI.transitLines(controller.signal), transactionAPI.transitStations(controller.signal)])
      .then(([lines, stationList]) => {
        setTransitLines(lines);
        setStations(stationList);
      })
      .catch(() => undefined);
    return () => controller.abort();
  }, [layerSettings.transit, transitLines]);

  // Station figures use the map's filters and time window but not its visible area, so
  // panning does not refetch them.
  const stationQuery = useMemo<TransactionQuery>(() => ({
    priceRange, areaRange, types: selectedTypes, includeSpecial, month: month ?? undefined,
  }), [priceRange, areaRange, selectedTypes, includeSpecial, month]);

  useEffect(() => {
    if (!layerSettings.transit || tab !== "map" || stationQuery.types.length === 0) return;
    const controller = new AbortController();
    const timer = window.setTimeout(() => {
      transactionAPI.stationYoYs(stationQuery, 500, controller.signal)
        .then((result) => setStationYoY(new Map(result.stations.map((station) => [station.stationId, station.priceYoY]))))
        .catch(() => undefined);
    }, 250);
    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [layerSettings.transit, tab, stationQuery]);

  const fetchStationTrend = useCallback(
    (stationId: string, signal: AbortSignal) => transactionAPI.stationTrend(stationQuery, stationId, signal),
    [stationQuery],
  );

  const fetchStationRent = useCallback((stationId: string, signal: AbortSignal) => transactionAPI.stationRent(stationId, signal), []);
  const fetchFacilities = useCallback((bbox: BBox, categories: string[], signal: AbortSignal) => transactionAPI.facilities(bbox, categories, signal), []);
  const fetchFacilityDensity = useCallback((bbox: BBox, resolution: 8 | 9, categories: string[], signal: AbortSignal) =>
    transactionAPI.facilityDensity(bbox, resolution, categories, signal), []);
  const fetchNearbyFacilities = useCallback((lat: number, lng: number, signal: AbortSignal) => transactionAPI.nearbyFacilities(lat, lng, 500, signal), []);
  const fetchPresaleProjects = useCallback((bbox: BBox, signal: AbortSignal) =>
    transactionAPI.presaleProjects({ city: "", months: 24, bbox }, signal).then((result) => result.projects), []);
  const fetchNearbyProjects = useCallback((lat: number, lng: number, signal: AbortSignal) =>
    transactionAPI.presaleProjects({ city: "", months: 24, near: { lat, lng, radius: 800 } }, signal).then((result) => result.projects), []);

  // The hexagon card uses the map's filters; a new function per query also resets its cache.
  const fetchCellDetail = useCallback((hex: string, signal: AbortSignal) => {
    if (!query) return Promise.reject(new Error("map not ready"));
    return transactionAPI.cellDetail(query, hex, signal);
  }, [query]);

  const viewDistrictSales = useCallback((cityCode: string, district: string) => {
    setFeed({ city: cityCode, district, latestOnly: false });
    setTab("feed");
  }, [setFeed, setTab]);

  const exportUrl = useMemo(() => {
    if (tab === "feed") return transactionAPI.exportUrl(feedQuery) || null;
    if (tab === "map" && query) {
      return transactionAPI.exportUrl({ ...query, from: query.month, to: query.month, sort: "date_desc" }) || null;
    }
    return null;
  }, [tab, feedQuery, query]);

  useEffect(() => {
    if (!query || tab !== "map") return;
    if (query.types.length === 0) {
      setTransactions([]);
      setHexData([]);
      setTrends([]);
      setSummary({ count: 0, avgPricePerPing: 0, medianTotalPrice: 0, yoy: null });
      setError(null);
      setIsLoading(false);
      return;
    }

    const controller = new AbortController();
    const timer = window.setTimeout(async () => {
      setIsLoading(true);
      setError(null);
      try {
        // The trend line always spans the 12-month window; the map and summary follow the time bar.
        const trendQuery = { ...query, month: undefined };
        const [mapData, trendData, summaryData] = await Promise.all([
          viewMode === "hex"
            ? transactionAPI.aggregateH3(query, h3Resolution, controller.signal)
            : transactionAPI.list(query, controller.signal),
          transactionAPI.trends(trendQuery, controller.signal),
          transactionAPI.summary(query, controller.signal),
        ]);
        if (viewMode === "hex") {
          setHexData(mapData as H3Aggregate[]);
          setTransactions([]);
        } else {
          setTransactions(mapData as TransactionData[]);
          setHexData([]);
        }
        setTrends(trendData);
        setSummary(summaryData);
      } catch (requestError) {
        if (!controller.signal.aborted) {
          setError(requestError instanceof Error ? requestError.message : "Unable to load data");
        }
      } finally {
        if (!controller.signal.aborted) setIsLoading(false);
      }
    }, 250);

    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [query, viewMode, h3Resolution, tab]);

  // Colour classes follow what is on screen, so both dense and quiet areas show contrast.
  const breaks = useMemo(() => priceBreaks(
    viewMode === "hex" ? hexData.map((hex) => hex.avgPricePerPing) : transactions.map((sale) => sale.pricePerPing),
  ), [viewMode, hexData, transactions]);

  const isEmpty = !isLoading && !error && summary?.count === 0;

  return (
    <div className="flex h-full w-full flex-col overflow-hidden bg-canvas font-sans text-slate-700">
      {IS_DEMO && <DemoBanner />}
      <TopBar tab={tab} setTab={setTab} onHelp={() => setIsHelpOpen(true)} exportUrl={exportUrl} showAdmin={isAdmin} />

      <div className="relative min-h-0 flex-1">
        {tab === "map" ? (
          <>
            <main className="absolute inset-0">
              <Suspense fallback={<MapLoadingFallback />}>
                <MapView
                  data={transactions}
                  hexData={hexData}
                  breaks={breaks}
                  viewMode={viewMode}
                  initialView={camera}
                  onCameraChange={handleCameraChange}
                  flyTarget={flyTarget}
                  layerSettings={layerSettings}
                  onLayerSettingsChange={updateLayerSettings}
                  districtBoundaries={districtBoundaries}
                  fetchCellDetail={fetchCellDetail}
                  onViewDistrict={viewDistrictSales}
                  windowLabel={month ? month.replace("-", "/") : t("hex.window12")}
                  transitLines={transitLines}
                  stations={stations}
                  stationYoY={stationYoY}
                  fetchStationTrend={fetchStationTrend}
                  fetchStationRent={fetchStationRent}
                  fetchNearbyProjects={fetchNearbyProjects}
                  fetchFacilities={fetchFacilities}
                  fetchFacilityDensity={fetchFacilityDensity}
                  fetchNearbyFacilities={fetchNearbyFacilities}
                  fetchPresaleProjects={fetchPresaleProjects}
                  onShowProject={showProject}
                  isPanelOpen={isPanelOpen}
                  focusStationId={focusStationId}
                  onFocusStationHandled={() => setFocusStationId(null)}
                  onSelectedStationChange={setStationId}
                />
              </Suspense>
            </main>

            <FilterPanel
              exportUrl={exportUrl}
              api={transactionAPI}
              onSelectPlace={handleSelectPlace}
              onEstimatePlace={estimatePlace}
              summary={summary}
              trends={trends}
              isLoading={isLoading}
              error={error}
              priceRange={priceRange}
              setPriceRange={setPriceRange}
              areaRange={areaRange}
              setAreaRange={setAreaRange}
              selectedTypes={selectedTypes}
              setSelectedTypes={setSelectedTypes}
              includeSpecial={includeSpecial}
              setIncludeSpecial={setIncludeSpecial}
              onReset={resetFilters}
              h3Resolution={h3Resolution}
              setH3Resolution={setH3Resolution}
              viewMode={viewMode}
              setViewMode={setViewMode}
              isOpen={isPanelOpen}
              setIsOpen={setIsPanelOpen}
            />

            {isEmpty && (
              <div className="pointer-events-none absolute left-1/2 top-4 z-20 -translate-x-1/2 rounded-full border border-slate-200 bg-white/95 px-4 py-2 text-xs text-slate-600 shadow-md lg:ml-40">
                {t("status.empty")}
              </div>
            )}

            <Legend breaks={breaks} showTransit={layerSettings.transit && stations.length > 0}
              density={layerSettings.facilityView === "density" && layerSettings.facilityGroups.length > 0}
              facilityGroups={layerSettings.facilityGroups} presales={layerSettings.presales} />
            <TimeBar months={months} month={month} setMonth={setMonth} isPlaying={isPlaying} setIsPlaying={setIsPlaying} />
          </>
        ) : tab === "analysis" ? (
          <Suspense fallback={<div className="flex h-full items-center justify-center text-sm text-slate-500">{t("status.loading")}</div>}>
            <AnalysisPage
              api={transactionAPI}
              priceRange={priceRange}
              setPriceRange={setPriceRange}
              areaRange={areaRange}
              setAreaRange={setAreaRange}
              selectedTypes={selectedTypes}
              setSelectedTypes={setSelectedTypes}
              includeSpecial={includeSpecial}
              setIncludeSpecial={setIncludeSpecial}
              onReset={resetFilters}
            />
          </Suspense>
        ) : tab === "districts" ? (
          <Suspense fallback={<div className="flex h-full items-center justify-center text-sm text-slate-500">{t("status.loading")}</div>}>
            <DistrictsPage
              api={transactionAPI}
              priceRange={priceRange}
              setPriceRange={setPriceRange}
              areaRange={areaRange}
              setAreaRange={setAreaRange}
              selectedTypes={selectedTypes}
              setSelectedTypes={setSelectedTypes}
              includeSpecial={includeSpecial}
              setIncludeSpecial={setIncludeSpecial}
              onReset={resetFilters}
            />
          </Suspense>
        ) : tab === "investment" ? (
          <Suspense fallback={<div className="flex h-full items-center justify-center text-sm text-slate-500">{t("status.loading")}</div>}>
            <InvestmentPage api={transactionAPI} state={investment} onChange={setInvestment} />
          </Suspense>
        ) : tab === "rents" ? (
          <Suspense fallback={<div className="flex h-full items-center justify-center text-sm text-slate-500">{t("status.loading")}</div>}>
            <RentsPage api={transactionAPI} state={rents} onChange={setRents} />
          </Suspense>
        ) : tab === "estimate" ? (
          <Suspense fallback={<div className="flex h-full items-center justify-center text-sm text-slate-500">{t("status.loading")}</div>}>
            {IS_DEMO ? <DemoUnavailable onAnalysis={() => setTab("analysis")} />
              : <EstimatePage api={transactionAPI} state={estimate} onChange={setEstimate} fetchNearbyFacilities={fetchNearbyFacilities} />}
          </Suspense>
        ) : tab === "presales" ? (
          <Suspense fallback={<div className="flex h-full items-center justify-center text-sm text-slate-500">{t("status.loading")}</div>}>
            <PresalesPage api={transactionAPI} state={presales} onChange={setPresales} onShowOnMap={showPoint} />
          </Suspense>
        ) : tab === "area" ? (
          <Suspense fallback={<div className="flex h-full items-center justify-center text-sm text-slate-500">{t("status.loading")}</div>}>
            <AreaPage api={transactionAPI} state={area} onChange={setArea} onShowProject={showProject} onShowStation={showStation} />
          </Suspense>
        ) : tab === "stations" ? (
          <Suspense fallback={<div className="flex h-full items-center justify-center text-sm text-slate-500">{t("status.loading")}</div>}>
            <StationsPage
              api={transactionAPI}
              priceRange={priceRange}
              setPriceRange={setPriceRange}
              areaRange={areaRange}
              setAreaRange={setAreaRange}
              selectedTypes={selectedTypes}
              setSelectedTypes={setSelectedTypes}
              includeSpecial={includeSpecial}
              setIncludeSpecial={setIncludeSpecial}
              onReset={resetFilters}
              onShowStation={showStation}
            />
          </Suspense>
        ) : tab === "feed" ? (
          <FeedPage
            exportUrl={exportUrl}
            api={transactionAPI}
            query={feedQuery}
            onShowOnMap={showOnMap}
            feed={feed}
            setFeed={setFeed}
            priceRange={priceRange}
            setPriceRange={setPriceRange}
            areaRange={areaRange}
            setAreaRange={setAreaRange}
            selectedTypes={selectedTypes}
            setSelectedTypes={setSelectedTypes}
            includeSpecial={includeSpecial}
            setIncludeSpecial={setIncludeSpecial}
            onReset={resetFeed}
          />
        ) : null}
      </div>

      {isHelpOpen && <HelpDialog onClose={() => setIsHelpOpen(false)} />}
    </div>
  );
}

function MapLoadingFallback() {
  return (
    <div className="flex h-full w-full items-center justify-center bg-canvas text-sm text-slate-400">
      <span className="mr-3 h-2.5 w-2.5 animate-pulse rounded-full bg-brand-500" />
      Loading map…
    </div>
  );
}
