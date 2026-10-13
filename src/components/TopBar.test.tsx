import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { LanguageProvider, translations } from "../lib/i18n";
import { NAVIGATION_GROUPS, navigationGroup, navigationHref } from "../lib/navigation";
import { TABS, parseAppState } from "../lib/url-state";
import TopBar from "./TopBar";
import ExportButton from "./ExportButton";

describe("grouped navigation", () => {
  // A community page is opened from other pages, never from the menu.
  const menuTabs = TABS.filter((tab) => tab !== "community");

  it("includes every existing page exactly once", () => {
    const pages = ["map", ...NAVIGATION_GROUPS.flatMap((group) => [...group.tabs])];
    expect([...pages].sort()).toEqual([...menuTabs].sort());
    expect(new Set(pages).size).toBe(menuTabs.length);
    expect(NAVIGATION_GROUPS.map((group) => group.tabs.length)).toEqual([4, 3, 2]);
  });

  it("keeps all existing deep links and category membership", () => {
    for (const tab of menuTabs) expect(parseAppState(navigationHref(tab), "").tab).toBe(tab);
    expect(parseAppState(navigationHref("community"), "?id=3d2801a712dc").tab).toBe("community");
    expect(navigationGroup("community")).toBe("region");
    expect(navigationGroup("map")).toBeUndefined();
    expect(navigationGroup("feed")).toBe("market");
    expect(navigationGroup("stations")).toBe("region");
    expect(navigationGroup("investment")).toBe("tools");
  });

  it("provides both languages for every category and page description", () => {
    for (const language of ["zh", "en"] as const) {
      for (const group of NAVIGATION_GROUPS) {
        expect(translations[language][`nav.group.${group.id}`]).toBeTruthy();
        for (const tab of group.tabs) expect(translations[language][`nav.description.${tab}`]).toBeTruthy();
      }
    }
  });

  it("renders accessible closed disclosures and no global export", () => {
    const html = renderToStaticMarkup(<LanguageProvider><TopBar tab="area" setTab={() => {}} onHelp={() => {}} showAdmin={false} exportUrl="/api/v1/export.csv" /></LanguageProvider>);
    expect(html).toContain('aria-controls="nav-region"');
    expect(html).toContain('aria-expanded="false"');
    expect(html).toContain('aria-labelledby="navigation-title"');
    expect(html).toContain('href="/area" aria-current="page"');
    expect(html).not.toContain('href="/admin"');
    expect(html).not.toContain("export.csv");
  });

  it("retains the admin link only for authorized users", () => {
    const html = renderToStaticMarkup(<LanguageProvider><TopBar tab="map" setTab={() => {}} onHelp={() => {}} showAdmin exportUrl={null} /></LanguageProvider>);
    expect(html).toContain('href="/admin"');
  });

  it("preserves the filtered CSV URL and disables unavailable exports", () => {
    const html = renderToStaticMarkup(<LanguageProvider><ExportButton url="/api/v1/export.csv?city=a&type=公寓" /></LanguageProvider>);
    expect(html).toContain('href="/api/v1/export.csv?city=a&amp;type=公寓"');
    expect(html).toContain('download=""');
    const disabled = renderToStaticMarkup(<LanguageProvider><ExportButton url={null} /></LanguageProvider>);
    expect(disabled).toContain('disabled=""');
    expect(disabled).not.toContain("href=");
  });
});
