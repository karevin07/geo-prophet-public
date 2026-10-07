import { useEffect, useState } from "react";
import { AlertCircle, CheckCircle2, ExternalLink, Info, Pencil, Plus, Trash2 } from "lucide-react";
import { adminAPI, type OpeningEdit, type Openings } from "../lib/api/admin";
import { useLanguage } from "../lib/i18n";
import { cn, fill } from "../lib/utils";
import { CARD, shortTime } from "./shared";

/** 2020-01-31 → 2020/01/31; null for lines that opened before 2015. */
function displayDate(value: string | null | undefined, before2015: string) {
  return value ? value.replaceAll("-", "/") : before2015;
}

/** Shows a URL as a link labelled with its host, anything else as text. */
function Source({ source }: { source: string }) {
  if (!source) return <span className="text-slate-300">—</span>;
  try {
    const url = new URL(source);
    if (url.protocol !== "http:" && url.protocol !== "https:") throw new Error("not a web link");
    return (
      <a href={url.href} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1 text-brand-700 hover:underline">
        {url.hostname.replace(/^www\./, "")}<ExternalLink className="h-3 w-3" />
      </a>
    );
  } catch {
    return <span className="text-slate-600">{source}</span>;
  }
}

function Updated({ by, at }: { by: string; at: string | null }) {
  if (!by) return <span className="text-slate-300">—</span>;
  return <span className="text-xs text-slate-500">{by}{at && ` · ${shortTime(at)}`}</span>;
}

interface Draft {
  kind: "line" | "platform";
  targetId: string;
  openedOn: string;
  source: string;
}

/** Edit line opening dates and platform exceptions; each save applies at once. */
export default function OpeningsPage() {
  const { t } = useLanguage();
  const [openings, setOpenings] = useState<Openings | null>(null);
  const [draft, setDraft] = useState<Draft | null>(null);
  const [adding, setAdding] = useState(false);
  const [deleting, setDeleting] = useState<string | null>(null);
  const [notice, setNotice] = useState<{ ok: boolean; text: string } | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    const controller = new AbortController();
    adminAPI.openings(controller.signal).then(setOpenings)
      .catch(() => !controller.signal.aborted && setNotice({ ok: false, text: t("admin.loadError") }));
    return () => controller.abort();
  }, [t]);

  const save = async (edit: OpeningEdit) => {
    setSaving(true);
    setNotice(null);
    try {
      setOpenings(await adminAPI.saveOpening(edit));
      setDraft(null);
      setAdding(false);
      setDeleting(null);
      setNotice({ ok: true, text: t("admin.openings.saved") });
    } catch (cause) {
      setNotice({ ok: false, text: fill(t("admin.openings.saveFailed"), { message: cause instanceof Error ? cause.message : "" }) });
    } finally {
      setSaving(false);
    }
  };
  const saveDraft = () => draft && save({ ...draft, openedOn: draft.openedOn || null });
  const editing = (kind: Draft["kind"], id: string) => draft?.kind === kind && draft.targetId === id && !adding;
  const before2015 = t("admin.openings.before2015");
  const input = "h-8 rounded-md border border-slate-200 px-2 text-sm text-slate-800 focus:border-brand-600 focus:outline-none";

  if (!openings) {
    return notice ? <Alert ok={false} text={notice.text} /> : <div className="h-96 animate-pulse rounded-2xl bg-slate-100" />;
  }

  return (
    <div className="space-y-5">
      <p className="flex items-start gap-2 rounded-xl border border-brand-100 bg-brand-50/70 px-4 py-3 text-sm text-brand-800">
        <Info className="mt-0.5 h-4 w-4 shrink-0" />
        <span>{t("admin.openings.note")} <span className="text-brand-700/80">{t("admin.openings.cacheNote")}</span></span>
      </p>
      {notice && <Alert ok={notice.ok} text={notice.text} />}

      <section className={cn(CARD, "p-4 sm:p-5")}>
        <h2 className="text-base font-semibold text-slate-900">{t("admin.openings.lines")}</h2>
        <div className="-mx-4 mt-3 overflow-x-auto px-4 sm:mx-0 sm:px-0">
          <table className="w-full min-w-[52rem] text-left text-sm">
            <thead>
              <tr className="border-b border-slate-100 text-xs text-slate-500">
                {["line", "system", "opened", "source", "updated"].map((column) => <th key={column} className="py-2 pr-3 font-medium">{t(`admin.openings.col.${column}`)}</th>)}
                <th className="w-40" />
              </tr>
            </thead>
            <tbody>
              {openings.lines.map((line) => editing("line", line.lineId) && draft ? (
                <tr key={line.lineId} className="border-b border-slate-100 bg-brand-50/40">
                  <td className="py-2 pr-3"><LineName name={line.name} id={line.lineId} color={line.color} /></td>
                  <td className="py-2 pr-3 text-xs text-slate-500">{t(`transit.system.${line.system}`)}</td>
                  <td className="py-2 pr-3"><input type="date" value={draft.openedOn} onChange={(event) => setDraft({ ...draft, openedOn: event.target.value })} className={input} /></td>
                  <td className="py-2 pr-3"><input value={draft.source} onChange={(event) => setDraft({ ...draft, source: event.target.value })} placeholder={t("admin.openings.sourcePlaceholder")} className={cn(input, "w-full min-w-[12rem]")} /></td>
                  <td className="py-2 pr-3"><Updated by={line.updatedBy} at={line.updatedAt} /></td>
                  <td className="py-2 text-right">
                    <div className="flex items-center justify-end gap-1.5">
                      <button onClick={saveDraft} disabled={saving} className="rounded-md bg-brand-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-brand-700 disabled:opacity-50">{t("admin.openings.save")}</button>
                      <button onClick={() => setDraft(null)} className="rounded-md px-2 py-1.5 text-xs text-slate-600 hover:bg-white">{t("admin.openings.cancel")}</button>
                      {line.openedOn && <button onClick={() => save({ kind: "line", targetId: line.lineId, openedOn: null, source: "" })} disabled={saving} className="px-1 text-xs text-slate-400 hover:text-red-600">{t("admin.openings.clear")}</button>}
                    </div>
                  </td>
                </tr>
              ) : (
                <tr key={line.lineId} className="border-b border-slate-100 last:border-0">
                  <td className="py-2.5 pr-3"><LineName name={line.name} id={line.lineId} color={line.color} /></td>
                  <td className="py-2.5 pr-3 text-xs text-slate-500">{t(`transit.system.${line.system}`)}</td>
                  <td className={cn("py-2.5 pr-3 tabular-nums", line.openedOn ? "text-slate-800" : "text-slate-400")}>{displayDate(line.openedOn, before2015)}</td>
                  <td className="py-2.5 pr-3"><Source source={line.source} /></td>
                  <td className="py-2.5 pr-3"><Updated by={line.updatedBy} at={line.updatedAt} /></td>
                  <td className="py-2.5 text-right">
                    <button onClick={() => { setAdding(false); setDraft({ kind: "line", targetId: line.lineId, openedOn: line.openedOn ?? "", source: line.source }); }}
                      className="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs text-brand-700 hover:bg-brand-50"><Pencil className="h-3.5 w-3.5" />{t("admin.openings.edit")}</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section className={cn(CARD, "p-4 sm:p-5")}>
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h2 className="text-base font-semibold text-slate-900">{t("admin.openings.platforms")}</h2>
            <p className="mt-1 text-xs text-slate-500">{t("admin.openings.platformsHint")}</p>
          </div>
          {!adding && (
            <button onClick={() => { setAdding(true); setDraft({ kind: "platform", targetId: "", openedOn: "", source: "" }); }}
              className="inline-flex items-center gap-1.5 rounded-lg bg-brand-600 px-3 py-2 text-sm font-medium text-white hover:bg-brand-700">
              <Plus className="h-4 w-4" />{t("admin.openings.add")}
            </button>
          )}
        </div>
        <div className="-mx-4 mt-3 overflow-x-auto px-4 sm:mx-0 sm:px-0">
          <table className="w-full min-w-[56rem] text-left text-sm">
            <thead>
              <tr className="border-b border-slate-100 text-xs text-slate-500">
                {["station", "line", "lineOpened", "stationOpened", "source", "updated"].map((column) => <th key={column} className="py-2 pr-3 font-medium">{t(`admin.openings.col.${column}`)}</th>)}
                <th className="w-32" />
              </tr>
            </thead>
            <tbody>
              {adding && draft && (
                <tr className="border-b border-slate-100 bg-brand-50/40">
                  <td className="py-2 pr-3" colSpan={3}>
                    <input list="platform-options" value={draft.targetId} onChange={(event) => setDraft({ ...draft, targetId: event.target.value.split(" ")[0] })}
                      placeholder={t("admin.openings.platformPlaceholder")} className={cn(input, "w-full")} />
                    <datalist id="platform-options">
                      {openings.options.map((option) => <option key={option.platformId} value={option.platformId}>{option.stationName}（{option.lines}）</option>)}
                    </datalist>
                  </td>
                  <td className="py-2 pr-3"><input type="date" value={draft.openedOn} onChange={(event) => setDraft({ ...draft, openedOn: event.target.value })} className={input} /></td>
                  <td className="py-2 pr-3" colSpan={2}><input value={draft.source} onChange={(event) => setDraft({ ...draft, source: event.target.value })} placeholder={t("admin.openings.sourcePlaceholder")} className={cn(input, "w-full")} /></td>
                  <td className="py-2 text-right">
                    <div className="flex justify-end gap-1.5">
                      <button onClick={saveDraft} disabled={saving || !draft.targetId || !draft.openedOn} className="rounded-md bg-brand-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-brand-700 disabled:opacity-50">{t("admin.openings.save")}</button>
                      <button onClick={() => { setAdding(false); setDraft(null); }} className="rounded-md px-2 py-1.5 text-xs text-slate-600 hover:bg-white">{t("admin.openings.cancel")}</button>
                    </div>
                  </td>
                </tr>
              )}
              {openings.platforms.map((platform) => editing("platform", platform.platformId) && draft ? (
                <tr key={platform.platformId} className="border-b border-slate-100 bg-brand-50/40">
                  <td className="py-2 pr-3"><LineName name={platform.stationName} id={platform.platformId} color={platform.lineColor} /></td>
                  <td className="py-2 pr-3 text-slate-600">{platform.lines}</td>
                  <td className="py-2 pr-3 tabular-nums text-slate-400">{displayDate(platform.lineOpenedOn, before2015)}</td>
                  <td className="py-2 pr-3"><input type="date" value={draft.openedOn} onChange={(event) => setDraft({ ...draft, openedOn: event.target.value })} className={input} /></td>
                  <td className="py-2 pr-3" colSpan={2}><input value={draft.source} onChange={(event) => setDraft({ ...draft, source: event.target.value })} placeholder={t("admin.openings.sourcePlaceholder")} className={cn(input, "w-full")} /></td>
                  <td className="py-2 text-right">
                    <div className="flex justify-end gap-1.5">
                      <button onClick={saveDraft} disabled={saving || !draft.openedOn} className="rounded-md bg-brand-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-brand-700 disabled:opacity-50">{t("admin.openings.save")}</button>
                      <button onClick={() => setDraft(null)} className="rounded-md px-2 py-1.5 text-xs text-slate-600 hover:bg-white">{t("admin.openings.cancel")}</button>
                    </div>
                  </td>
                </tr>
              ) : (
                <tr key={platform.platformId} className="border-b border-slate-100 last:border-0">
                  <td className="py-2.5 pr-3"><LineName name={platform.stationName} id={platform.platformId} color={platform.lineColor} /></td>
                  <td className="py-2.5 pr-3 text-slate-600">{platform.lines}</td>
                  <td className="py-2.5 pr-3 tabular-nums text-slate-400">{displayDate(platform.lineOpenedOn, before2015)}</td>
                  <td className="py-2.5 pr-3 font-medium tabular-nums text-slate-800">{displayDate(platform.openedOn, before2015)}</td>
                  <td className="py-2.5 pr-3"><Source source={platform.source} /></td>
                  <td className="py-2.5 pr-3"><Updated by={platform.updatedBy} at={platform.updatedAt} /></td>
                  <td className="py-2.5 text-right">
                    {deleting === platform.platformId ? (
                      <div className="inline-flex items-center gap-1.5 text-xs">
                        <span className="max-w-[10rem] text-left text-slate-600">{fill(t("admin.openings.confirmDelete"), { station: platform.stationName })}</span>
                        <button onClick={() => save({ kind: "platform", targetId: platform.platformId, openedOn: null, source: "" })} disabled={saving} className="rounded-md bg-red-600 px-2 py-1 font-medium text-white">{t("admin.openings.delete")}</button>
                        <button onClick={() => setDeleting(null)} className="px-1 text-slate-500">{t("admin.openings.cancel")}</button>
                      </div>
                    ) : (
                      <div className="inline-flex gap-1">
                        <button onClick={() => { setAdding(false); setDraft({ kind: "platform", targetId: platform.platformId, openedOn: platform.openedOn, source: platform.source }); }}
                          aria-label={t("admin.openings.edit")} title={t("admin.openings.edit")} className="rounded-md p-1.5 text-slate-500 hover:bg-slate-100 hover:text-brand-700"><Pencil className="h-3.5 w-3.5" /></button>
                        <button onClick={() => setDeleting(platform.platformId)}
                          aria-label={t("admin.openings.delete")} title={t("admin.openings.delete")} className="rounded-md p-1.5 text-slate-500 hover:bg-red-50 hover:text-red-600"><Trash2 className="h-3.5 w-3.5" /></button>
                      </div>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}

function LineName({ name, id, color }: { name: string; id: string; color: string }) {
  return (
    <span className="flex items-center gap-2 whitespace-nowrap">
      <span className="h-2.5 w-2.5 shrink-0 rounded-full" style={{ backgroundColor: color }} />
      <span className="font-medium text-slate-900">{name}</span>
      <span className="font-mono text-xs text-slate-400">{id}</span>
    </span>
  );
}

function Alert({ ok, text }: { ok: boolean; text: string }) {
  const Icon = ok ? CheckCircle2 : AlertCircle;
  return (
    <p role={ok ? "status" : "alert"} className={cn("flex items-center gap-2 rounded-xl border px-4 py-2.5 text-sm", ok ? "border-emerald-200 bg-emerald-50 text-emerald-800" : "border-red-200 bg-red-50 text-red-700")}>
      <Icon className="h-4 w-4 shrink-0" />{text}
    </p>
  );
}
