import { afterEach, describe, expect, it, vi } from "vitest";

// The demo's settings are read when modules load, so each case loads them afresh.
async function load(env: Record<string, string>) {
  vi.resetModules();
  for (const [name, value] of Object.entries(env)) vi.stubEnv(name, value);
  return {
    urlState: await import("./url-state"),
    navigation: await import("./navigation"),
  };
}

afterEach(() => vi.unstubAllEnvs());

describe("online demo", () => {
  it("serves every page under the base path", async () => {
    const { urlState, navigation } = await load({ BASE_URL: "/geo-prophet-public/", VITE_DATA_SOURCE: "snapshot" });
    expect(navigation.navigationHref("map")).toBe("/geo-prophet-public/");
    expect(navigation.navigationHref("area")).toBe("/geo-prophet-public/area");
    expect(urlState.parseAppState("/geo-prophet-public/area", "").tab).toBe("area");
    expect(urlState.parseAppState("/geo-prophet-public", "").tab).toBe("map");
    expect(urlState.formatAppState({ ...urlState.DEFAULT_STATE, tab: "feed" })).toMatch(/^\/geo-prophet-public\/feed\?/);
  });

  it("leaves out pages the snapshot cannot serve", async () => {
    const { urlState, navigation } = await load({ BASE_URL: "/geo-prophet-public/", VITE_DATA_SOURCE: "snapshot" });
    expect(urlState.parseAppState("/geo-prophet-public/rents", "").tab).toBe("map");
    expect(navigation.VISIBLE_NAVIGATION_GROUPS.flatMap((group) => group.tabs)).toEqual(["feed", "analysis", "area", "districts", "estimate"]);
  });

  it("keeps every page at the root otherwise", async () => {
    const { urlState, navigation } = await load({ BASE_URL: "/", VITE_DATA_SOURCE: "" });
    expect(navigation.navigationHref("map")).toBe("/");
    expect(urlState.parseAppState("/rents", "").tab).toBe("rents");
    expect(navigation.VISIBLE_NAVIGATION_GROUPS).toEqual(navigation.NAVIGATION_GROUPS.map((group) => ({ id: group.id, tabs: [...group.tabs] })));
  });
});
