import { afterEach, describe, expect, it, vi } from "vitest";
import { DEFAULT_LAYER_SETTINGS, loadLayerSettings, saveLayerSettings } from "./layer-settings";

describe("layer settings", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("round-trips and clamps stored values", () => {
    const store = new Map<string, string>();
    vi.stubGlobal("localStorage", { getItem: (key: string) => store.get(key) ?? null, setItem: (key: string, value: string) => store.set(key, value) });
    const settings = { opacity: 0.7, basemap: "light", districts: true, transit: false, facilityGroups: ["education", "daily"], facilityView: "density", presales: true } as const;
    saveLayerSettings({ ...settings, facilityGroups: [...settings.facilityGroups] });
    // Groups come back in display order.
    expect(loadLayerSettings()).toEqual({ ...settings, facilityGroups: ["daily", "education"] });
    store.set("geoprophet.layers", JSON.stringify({ opacity: 5, basemap: "satellite", districts: "yes", facilityGroups: ["casinos"], facilityView: "x" }));
    expect(loadLayerSettings()).toEqual({ ...DEFAULT_LAYER_SETTINGS, opacity: 0.9 });
  });

  it("falls back to defaults when storage is unavailable", () => {
    vi.stubGlobal("localStorage", { getItem: () => { throw new Error("blocked"); }, setItem: () => { throw new Error("blocked"); } });
    expect(loadLayerSettings()).toEqual(DEFAULT_LAYER_SETTINGS);
    expect(() => saveLayerSettings(DEFAULT_LAYER_SETTINGS)).not.toThrow();
  });
});
