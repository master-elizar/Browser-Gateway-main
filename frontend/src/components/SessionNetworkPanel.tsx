import { useMemo, useState, type Dispatch, type SetStateAction } from "react";
import { useTranslation } from "react-i18next";
import type { NetworkEventItem, TIResult } from "../api/client";

export type NetTab = "all" | "dns" | "http" | "url" | "fqdn" | "ip" | "ti" | "tree" | "netflow";
export type NetSort = "time_desc" | "time_asc" | "host_asc" | "host_desc" | "status_asc" | "status_desc" | "method_asc" | "url_asc";
export type StatusFilter = "all" | "2xx" | "3xx" | "4xx" | "5xx" | "error" | "dns_only";

function hostOf(urlOrHost?: string): string {
  if (!urlOrHost) return "";
  try {
    if (urlOrHost.includes("://")) return new URL(urlOrHost).hostname;
  } catch {
    /* ignore */
  }
  return urlOrHost.split("/")[0] || "";
}

function isIP(host: string): boolean {
  return /^(\d{1,3}\.){3}\d{1,3}$/.test(host) || host.includes(":");
}

function tiKey(value?: string, kind?: string): string {
  if (!value) return "";
  const v = value.trim().toLowerCase();
  if (kind === "url" || v.includes("://")) return v;
  const h = hostOf(v);
  return (h || v).replace(/^\[|\]$/g, "");
}

function primaryIndicator(
  ev: NetworkEventItem,
  tab: NetTab,
): { value: string; kind: "domain" | "ip" | "url" } | null {
  if (ev.type === "ti" && ev.indicator) {
    const k = (ev.kind as "domain" | "ip" | "url") || "domain";
    return { value: ev.indicator, kind: k };
  }
  if (tab === "ip" || ev.type === "ip") {
    const ip = (ev.answers && ev.answers[0]) || ev.remoteIP || "";
    if (ip && isIP(ip)) return { value: ip, kind: "ip" };
    const h = hostOf(ev.url || ev.query);
    if (h && isIP(h)) return { value: h, kind: "ip" };
    return null;
  }
  if (tab === "url") {
    if (ev.url) return { value: ev.url, kind: "url" };
    const h = hostOf(ev.query);
    return h ? { value: h, kind: isIP(h) ? "ip" : "domain" } : null;
  }
  if (tab === "fqdn" || tab === "dns") {
    const d = (ev.query || hostOf(ev.url) || "").replace(/\.$/, "");
    if (!d) return null;
    return { value: d, kind: isIP(d) ? "ip" : "domain" };
  }
  // all / http / netflow — prefer host, fall back to URL
  const host = hostOf(ev.url || ev.query);
  if (host) return { value: host, kind: isIP(host) ? "ip" : "domain" };
  if (ev.url) return { value: ev.url, kind: "url" };
  if (ev.query) return { value: ev.query, kind: isIP(ev.query) ? "ip" : "domain" };
  return null;
}

function resolveTI(
  map: Record<string, TIResult>,
  ind: { value: string; kind: string } | null,
  ev?: NetworkEventItem,
): TIResult | undefined {
  if (ev?.type === "ti") {
    const r = eventToTIResult(ev);
    if (r) return r;
  }
  if (!ind) return undefined;
  return (
    map[tiKey(ind.value, ind.kind)] ||
    map[tiKey(ind.value)] ||
    map[tiKey(hostOf(ind.value))] ||
    undefined
  );
}

function eventToTIResult(ev: NetworkEventItem): TIResult | null {
  if (ev.type !== "ti" || !ev.indicator) return null;
  return {
    provider: ev.provider || "multi",
    kind: ev.kind || "domain",
    indicator: ev.indicator,
    verdict: ev.verdict || "unknown",
    malicious: ev.malicious ?? 0,
    suspicious: ev.suspicious ?? 0,
    harmless: ev.harmless ?? 0,
    undetected: ev.undetected ?? 0,
    permalink: ev.permalink,
    cached: ev.cached,
    checkedAt: ev.ts,
    providers: ev.providers,
  };
}

function parsePatterns(raw: string): string[] {
  return raw
    .split(/[,;\n]+/)
    .map((s) => s.trim().toLowerCase())
    .filter(Boolean);
}

function eventHaystack(ev: NetworkEventItem): string {
  return [
    ev.type,
    ev.method,
    ev.status != null ? String(ev.status) : "",
    ev.url,
    ev.query,
    ev.remoteIP,
    ev.indicator,
    ev.verdict,
    ev.documentURL,
    ev.resourceType,
    ev.initiator?.url,
    ...(ev.answers || []),
    hostOf(ev.url || ev.query || ev.indicator),
  ]
    .filter(Boolean)
    .join(" ")
    .toLowerCase();
}

function matchesPatterns(haystack: string, patterns: string[], mode: "include" | "exclude"): boolean {
  if (patterns.length === 0) return mode === "include";
  const hit = patterns.some((p) => {
    if (p.startsWith("/") && p.endsWith("/") && p.length > 2) {
      try {
        return new RegExp(p.slice(1, -1), "i").test(haystack);
      } catch {
        return haystack.includes(p);
      }
    }
    if (p.includes("*")) {
      const re = new RegExp("^" + p.split("*").map(escapeRe).join(".*") + "$", "i");
      return re.test(haystack) || haystack.includes(p.replace(/\*/g, ""));
    }
    return haystack.includes(p);
  });
  return mode === "include" ? hit : !hit;
}

function escapeRe(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function statusMatches(ev: NetworkEventItem, status: StatusFilter): boolean {
  if (status === "all") return true;
  if (status === "dns_only") return ev.type === "dns";
  const code = ev.status;
  if (code == null) return status === "error" ? ev.type === "http" : false;
  if (status === "2xx") return code >= 200 && code < 300;
  if (status === "3xx") return code >= 300 && code < 400;
  if (status === "4xx") return code >= 400 && code < 500;
  if (status === "5xx") return code >= 500 && code < 600;
  if (status === "error") return code >= 400;
  return true;
}

function tabEvents(events: NetworkEventItem[], tab: NetTab): NetworkEventItem[] {
  switch (tab) {
    case "all":
      return events;
    case "dns":
      return events.filter((e) => e.type === "dns");
    case "http":
      return events.filter((e) => e.type === "http");
    case "url":
      return events.filter((e) => e.type === "http" && !!e.url);
    case "fqdn": {
      const seen = new Set<string>();
      const out: NetworkEventItem[] = [];
      for (const e of events) {
        if (e.type === "ti") continue;
        const host = hostOf(e.url || e.query);
        if (!host || isIP(host) || seen.has(host)) continue;
        seen.add(host);
        out.push({ ...e, type: "fqdn", query: host });
      }
      return out;
    }
    case "ip": {
      const seen = new Set<string>();
      const out: NetworkEventItem[] = [];
      const pushIp = (ip: string, e: NetworkEventItem, query?: string) => {
        if (!ip || !isIP(ip) || seen.has(ip)) return;
        seen.add(ip);
        out.push({
          ...e,
          type: "ip",
          query: query || e.query || hostOf(e.url) || ip,
          answers: [ip],
          remoteIP: ip,
        });
      };
      for (const e of events) {
        if (e.type === "ti") continue;
        for (const ip of e.answers || []) {
          pushIp(ip, e, e.query || hostOf(e.url));
        }
        if (e.remoteIP) {
          pushIp(e.remoteIP, e, hostOf(e.url || e.query));
        }
        const host = hostOf(e.url || e.query);
        if (isIP(host)) {
          pushIp(host, e, host);
        }
      }
      return out;
    }
    case "ti":
      return events.filter((e) => e.type === "ti");
    case "tree":
      return events.filter((e) => e.type === "http");
    case "netflow":
      return events.filter((e) => e.type === "http" || e.type === "dns");
    default:
      return events;
  }
}

function sortEvents(events: NetworkEventItem[], sort: NetSort): NetworkEventItem[] {
  const out = events.slice();
  const ts = (e: NetworkEventItem) => (e.ts ? Date.parse(e.ts) || 0 : 0);
  const host = (e: NetworkEventItem) => hostOf(e.url || e.query).toLowerCase();
  out.sort((a, b) => {
    switch (sort) {
      case "time_asc":
        return ts(a) - ts(b);
      case "time_desc":
        return ts(b) - ts(a);
      case "host_asc":
        return host(a).localeCompare(host(b));
      case "host_desc":
        return host(b).localeCompare(host(a));
      case "status_asc":
        return (a.status ?? 9999) - (b.status ?? 9999);
      case "status_desc":
        return (b.status ?? -1) - (a.status ?? -1);
      case "method_asc":
        return (a.method || "").localeCompare(b.method || "");
      case "url_asc":
        return (a.url || a.query || "").localeCompare(b.url || b.query || "");
      default:
        return 0;
    }
  });
  return out;
}

function buildNetworkView(
  events: NetworkEventItem[],
  opts: {
    tab: NetTab;
    include: string;
    exclude: string;
    sort: NetSort;
    status: StatusFilter;
    method: string;
  },
): NetworkEventItem[] {
  const include = parsePatterns(opts.include);
  const exclude = parsePatterns(opts.exclude);
  let list = tabEvents(events, opts.tab);
  list = list.filter((ev) => statusMatches(ev, opts.status));
  if (opts.method !== "all") {
    list = list.filter((ev) => (ev.method || "").toUpperCase() === opts.method.toUpperCase());
  }
  if (include.length) {
    list = list.filter((ev) => matchesPatterns(eventHaystack(ev), include, "include"));
  }
  if (exclude.length) {
    list = list.filter((ev) => matchesPatterns(eventHaystack(ev), exclude, "exclude"));
  }
  return sortEvents(list, opts.sort);
}

type TreeHostNode = {
  host: string;
  requests: NetworkEventItem[];
};

type TreePageNode = {
  key: string;
  documentURL: string;
  host: string;
  document?: NetworkEventItem;
  hosts: TreeHostNode[];
  requestCount: number;
};

function refererOf(ev: NetworkEventItem): string {
  const h = ev.requestHeaders || {};
  return h.Referer || h.referer || "";
}

function docGroupKey(url: string): string {
  if (!url || url === "(unknown)") return url || "(unknown)";
  try {
    const u = new URL(url);
    return `${u.origin}${u.pathname}`;
  } catch {
    return url;
  }
}

function buildRequestTree(events: NetworkEventItem[]): TreePageNode[] {
  const http = events.filter((e) => e.type === "http" && !!e.url);
  const pages = new Map<string, TreePageNode>();
  const pageHostReqs = new Map<string, Map<string, NetworkEventItem[]>>();

  const ensurePage = (documentURL: string, docEv?: NetworkEventItem): TreePageNode => {
    const key = docGroupKey(documentURL);
    let page = pages.get(key);
    if (!page) {
      page = {
        key,
        documentURL,
        host: hostOf(documentURL) || documentURL,
        document: docEv,
        hosts: [],
        requestCount: 0,
      };
      pages.set(key, page);
    } else if (docEv && !page.document) {
      page.document = docEv;
    } else if (docEv && page.document) {
      const ta = page.document.ts ? Date.parse(page.document.ts) : 0;
      const tb = docEv.ts ? Date.parse(docEv.ts) : 0;
      if (tb >= ta) page.document = docEv;
    }
    return page;
  };

  for (const ev of http) {
    if ((ev.resourceType || "").toLowerCase() === "document") {
      ensurePage(ev.url || ev.documentURL || "", ev);
    }
  }

  for (const ev of http) {
    let pageURL = (ev.documentURL || "").trim();
    if (!pageURL) pageURL = refererOf(ev);
    if (!pageURL && (ev.resourceType || "").toLowerCase() === "document") {
      pageURL = ev.url || "";
    }
    if (!pageURL && ev.initiator?.url) pageURL = ev.initiator.url;
    if (!pageURL) pageURL = "(unknown)";

    const page = ensurePage(
      pageURL,
      (ev.resourceType || "").toLowerCase() === "document" ? ev : undefined,
    );

    if ((ev.resourceType || "").toLowerCase() === "document" && docGroupKey(ev.url || "") === page.key) {
      continue;
    }

    const h = hostOf(ev.url);
    if (!h) continue;

    let hostMap = pageHostReqs.get(page.key);
    if (!hostMap) {
      hostMap = new Map();
      pageHostReqs.set(page.key, hostMap);
    }
    const list = hostMap.get(h) || [];
    list.push(ev);
    hostMap.set(h, list);
  }

  const out: TreePageNode[] = [];
  for (const page of pages.values()) {
    const hostMap = pageHostReqs.get(page.key) || new Map();
    const hosts: TreeHostNode[] = [];
    let count = 0;
    for (const [host, reqs] of hostMap) {
      hosts.push({ host, requests: reqs });
      count += reqs.length;
    }
    hosts.sort((a, b) => a.host.localeCompare(b.host));
    page.hosts = hosts;
    page.requestCount = count;
    out.push(page);
  }

  out.sort((a, b) => {
    const ta = a.document?.ts ? Date.parse(a.document.ts) : 0;
    const tb = b.document?.ts ? Date.parse(b.document.ts) : 0;
    return tb - ta;
  });
  return out;
}

function RequestTreeView({
  pages,
  expanded,
  setExpanded,
  tiMap,
  lookupBusy,
  onLookup,
}: {
  pages: TreePageNode[];
  expanded: Record<string, boolean>;
  setExpanded: Dispatch<SetStateAction<Record<string, boolean>>>;
  tiMap: Record<string, TIResult>;
  lookupBusy: string | null;
  onLookup?: (value: string, kind?: string) => void;
}) {
  const { t } = useTranslation();

  function isOpen(key: string, defaultOpen: boolean): boolean {
    if (key in expanded) return expanded[key];
    return defaultOpen;
  }

  function toggle(key: string, defaultOpen: boolean) {
    setExpanded((prev) => ({ ...prev, [key]: !isOpen(key, defaultOpen) }));
  }

  if (pages.length === 0) {
    return (
      <div className="rounded-lg border border-dashed border-[var(--color-line)] px-3 py-8 text-center text-sm text-[var(--color-fog)]">
        {t("viewer.treeEmpty")}
      </div>
    );
  }

  return (
    <div className="space-y-2">
      {pages.map((page) => {
        const pageOpen = isOpen(`p:${page.key}`, true);
        const pageInd = page.host
          ? ({ value: page.host, kind: isIP(page.host) ? "ip" : "domain" } as const)
          : null;
        const pageTi = resolveTI(tiMap, pageInd, page.document);
        const pageBusy = pageInd ? lookupBusy === tiKey(pageInd.value, pageInd.kind) : false;
        return (
          <div
            key={page.key}
            className="rounded-lg border border-[var(--color-line)]/70 bg-[var(--color-ink)]/40 text-xs"
          >
            <div className="flex items-start gap-2 px-3 py-2">
              <button
                type="button"
                className="mt-0.5 w-4 shrink-0 font-mono text-[var(--color-fog)]"
                onClick={() => toggle(`p:${page.key}`, true)}
                aria-label="toggle"
              >
                {pageOpen ? "▾" : "▸"}
              </button>
              <div className="min-w-0 flex-1">
                <div className="font-mono text-[10px] uppercase tracking-wide text-[var(--color-signal-2)]">
                  {t("viewer.treePage")} · {page.requestCount} {t("viewer.treeReqs")} · {page.hosts.length}{" "}
                  {t("viewer.treeHosts")}
                </div>
                <div className="break-all text-[var(--color-snow)]">
                  {page.documentURL === "(unknown)" ? t("viewer.treeUnknownPage") : page.documentURL}
                </div>
              </div>
              {pageInd && (
                <TICheck
                  ti={pageTi}
                  busy={pageBusy}
                  onCheck={onLookup ? () => onLookup(pageInd.value, pageInd.kind) : undefined}
                />
              )}
            </div>
            {pageOpen && (
              <div className="space-y-1 border-t border-[var(--color-line)]/50 px-2 py-2 pl-7">
                {page.hosts.length === 0 ? (
                  <div className="px-2 py-1 text-[var(--color-fog)]">{t("viewer.treeNoChildren")}</div>
                ) : (
                  page.hosts.map((node) => {
                    const hostKey = `h:${page.key}:${node.host}`;
                    const hostOpen = isOpen(hostKey, false);
                    const hostInd = {
                      value: node.host,
                      kind: (isIP(node.host) ? "ip" : "domain") as "ip" | "domain",
                    };
                    const hostTi = resolveTI(tiMap, hostInd);
                    const hostBusy = lookupBusy === tiKey(hostInd.value, hostInd.kind);
                    return (
                      <div
                        key={hostKey}
                        className="rounded-md border border-[var(--color-line)]/40 bg-[var(--color-panel)]/30"
                      >
                        <div className="flex items-start gap-2 px-2 py-1.5">
                          <button
                            type="button"
                            className="mt-0.5 w-4 shrink-0 font-mono text-[var(--color-fog)]"
                            onClick={() => toggle(hostKey, false)}
                          >
                            {hostOpen ? "▾" : "▸"}
                          </button>
                          <div className="min-w-0 flex-1 font-mono text-[var(--color-snow)]">
                            {node.host}
                            <span className="ml-2 text-[10px] text-[var(--color-fog)]">
                              {node.requests.length}
                            </span>
                          </div>
                          <TICheck
                            ti={hostTi}
                            busy={hostBusy}
                            onCheck={onLookup ? () => onLookup(hostInd.value, hostInd.kind) : undefined}
                          />
                        </div>
                        {hostOpen && (
                          <div className="space-y-1 border-t border-[var(--color-line)]/30 px-2 py-1.5 pl-6">
                            {node.requests.map((ev, idx) => {
                              const ind = primaryIndicator(ev, "url");
                              const ti = resolveTI(tiMap, ind, ev);
                              const busyKey = ind ? tiKey(ind.value, ind.kind) : "";
                              return (
                                <div
                                  key={`${ev.id || ev.ts || idx}-${ev.url}`}
                                  className="flex items-start gap-2 rounded px-1 py-1 hover:bg-[var(--color-ink)]/50"
                                >
                                  <div className="min-w-0 flex-1">
                                    <div className="font-mono text-[10px] text-[var(--color-signal-2)]">
                                      {[ev.method, ev.status, ev.resourceType].filter(Boolean).join(" · ")}
                                    </div>
                                    <div className="break-all text-[var(--color-fog)]">{ev.url}</div>
                                  </div>
                                  {ind && (
                                    <TICheck
                                      ti={ti}
                                      busy={lookupBusy === busyKey}
                                      onCheck={
                                        onLookup ? () => onLookup(ind.value, ind.kind) : undefined
                                      }
                                    />
                                  )}
                                </div>
                              );
                            })}
                          </div>
                        )}
                      </div>
                    );
                  })
                )}
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}

function TIBadge({ ti }: { ti?: TIResult | null }) {
  if (!ti || !ti.verdict) return null;
  const tone =
    ti.verdict === "malicious"
      ? "border-[var(--color-danger)]/50 bg-[var(--color-danger)]/10 text-[var(--color-danger)]"
      : ti.verdict === "suspicious"
        ? "border-amber-500/50 bg-amber-500/10 text-amber-300"
        : ti.verdict === "clean"
          ? "border-emerald-500/50 bg-emerald-500/10 text-emerald-300"
          : "border-[var(--color-line)] text-[var(--color-fog)]";
  const total =
    (ti.malicious ?? 0) + (ti.suspicious ?? 0) + (ti.harmless ?? 0) + (ti.undetected ?? 0);
  const hits = ti.malicious ?? 0;
  const label = total > 0 ? `${ti.verdict} ${hits}/${total}` : ti.verdict;
  const breakdown =
    ti.providers && ti.providers.length
      ? ti.providers
          .map((p) => `${p.provider}:${p.error ? "error" : p.verdict}`)
          .join(" · ")
      : `${ti.provider}: ${ti.indicator}`;
  const cls = `inline-flex shrink-0 items-center rounded border px-1.5 py-0.5 font-mono text-[10px] uppercase tracking-wide ${tone}`;
  if (ti.permalink) {
    return (
      <a href={ti.permalink} target="_blank" rel="noreferrer" className={cls} title={breakdown}>
        {label}
      </a>
    );
  }
  return (
    <span className={cls} title={breakdown}>
      {label}
    </span>
  );
}

/** Check button → after click shows verdict badge next to a quiet re-check. */
function TICheck({
  ti,
  busy,
  onCheck,
}: {
  ti?: TIResult | null;
  busy?: boolean;
  onCheck?: () => void;
}) {
  const { t } = useTranslation();
  if (!onCheck && !ti) return null;

  if (busy) {
    return (
      <span className="inline-flex shrink-0 rounded border border-[var(--color-signal)]/40 px-2 py-0.5 font-mono text-[10px] text-[var(--color-signal-2)]">
        {t("viewer.tiLookingUp")}
      </span>
    );
  }

  if (ti?.verdict && !ti.error) {
    return (
      <span className="inline-flex shrink-0 items-center gap-1">
        <TIBadge ti={ti} />
        {onCheck && (
          <button
            type="button"
            onClick={onCheck}
            className="rounded border border-[var(--color-line)] px-1 py-0.5 text-[10px] text-[var(--color-fog)] hover:text-[var(--color-snow)]"
            title={t("viewer.tiRecheck")}
          >
            ↻
          </button>
        )}
      </span>
    );
  }

  if (!onCheck) return null;

  return (
    <button
      type="button"
      onClick={onCheck}
      className="inline-flex shrink-0 items-center rounded border border-[var(--color-signal)]/50 bg-[var(--color-signal-dim)] px-2 py-0.5 text-[10px] font-medium uppercase tracking-wide text-[var(--color-signal-2)] hover:border-[var(--color-signal)]"
    >
      {t("viewer.tiCheck")}
    </button>
  );
}

function NetworkRow({
  ev,
  tab,
  ti,
  lookupBusy,
  onLookup,
}: {
  ev: NetworkEventItem;
  tab: NetTab;
  ti?: TIResult;
  lookupBusy?: boolean;
  onLookup?: () => void;
}) {
  const host = hostOf(ev.url || ev.query || ev.indicator);
  let title = ev.url || ev.query || ev.indicator || "—";
  let meta = [ev.type, ev.method, ev.status].filter((x) => x != null && x !== "").join(" · ");

  if (tab === "fqdn") {
    title = ev.query || host;
    meta = "FQDN";
  } else if (tab === "ip") {
    title = (ev.answers && ev.answers[0]) || host;
    meta = ev.query ? `IP ← ${ev.query}` : "IP";
  } else if (tab === "dns") {
    title = ev.query || "—";
    meta = `${ev.qtype || "A"}${(ev.answers || []).length ? ` → ${(ev.answers || []).join(", ")}` : ""}`;
  } else if (tab === "url") {
    title = ev.url || "—";
    meta = `${ev.method || "GET"} · ${ev.status ?? "…"}`;
  } else if (tab === "ti" || ev.type === "ti") {
    title = ev.indicator || "—";
    meta = `${ev.provider || "VT"} · ${ev.kind || "indicator"}`;
  }

  return (
    <div className="rounded-lg border border-[var(--color-line)]/70 bg-[var(--color-ink)]/40 px-3 py-2 text-xs">
      <div className="mb-1 font-mono text-[var(--color-signal-2)]">{meta}</div>
      <div className="flex items-start gap-2">
        <div className="min-w-0 flex-1 break-all text-[var(--color-snow)]">{title}</div>
        <TICheck ti={ti} busy={lookupBusy} onCheck={onLookup} />
      </div>
      {ev.ts && <div className="mt-1 font-mono text-[10px] text-[var(--color-fog)]">{ev.ts}</div>}
    </div>
  );
}


export type SessionNetworkPanelProps = {
  events: NetworkEventItem[];
  tiOverlay?: Record<string, TIResult>;
  /** History / read-only: same UI, no per-row Check, Enrich/Clear disabled. */
  readOnly?: boolean;
  enrichBusy?: boolean;
  clearBusy?: boolean;
  lookupBusy?: string | null;
  onEnrich?: () => void;
  onClearTab?: (tab: NetTab) => void;
  onClearAll?: () => void;
  onLookup?: (value: string, kind?: string) => void;
};

export function SessionNetworkPanel({
  events,
  tiOverlay = {},
  readOnly = false,
  enrichBusy = false,
  clearBusy = false,
  lookupBusy = null,
  onEnrich,
  onClearTab,
  onClearAll,
  onLookup,
}: SessionNetworkPanelProps) {
  const { t } = useTranslation();
  const [netTab, setNetTab] = useState<NetTab>("all");
  const [netInclude, setNetInclude] = useState("");
  const [netExclude, setNetExclude] = useState("");
  const [netSort, setNetSort] = useState<NetSort>("time_desc");
  const [netStatus, setNetStatus] = useState<StatusFilter>("all");
  const [netMethod, setNetMethod] = useState("all");
  const [treeExpanded, setTreeExpanded] = useState<Record<string, boolean>>({});

  const tiMap = useMemo(() => {
    const m: Record<string, TIResult> = { ...tiOverlay };
    for (const ev of events) {
      const r = eventToTIResult(ev);
      if (!r) continue;
      const k = tiKey(r.indicator);
      if (k) m[k] = r;
    }
    return m;
  }, [events, tiOverlay]);

  const filtered = useMemo(
    () =>
      buildNetworkView(events, {
        tab: netTab === "tree" ? "http" : netTab,
        include: netInclude,
        exclude: netExclude,
        sort: netSort,
        status: netStatus,
        method: netMethod,
      }),
    [events, netTab, netInclude, netExclude, netSort, netStatus, netMethod],
  );

  const requestTree = useMemo(() => {
    if (netTab !== "tree") return [];
    return buildRequestTree(filtered);
  }, [netTab, filtered]);

  const tabCounts = useMemo(() => {
    const tabs: NetTab[] = ["all", "dns", "http", "url", "fqdn", "ip", "ti", "tree", "netflow"];
    const out: Record<NetTab, number> = {
      all: 0, dns: 0, http: 0, url: 0, fqdn: 0, ip: 0, ti: 0, tree: 0, netflow: 0,
    };
    for (const tab of tabs) {
      if (tab === "tree") {
        const http = buildNetworkView(events, {
          tab: "http",
          include: netInclude,
          exclude: netExclude,
          sort: netSort,
          status: netStatus,
          method: netMethod,
        });
        out.tree = buildRequestTree(http).length;
        continue;
      }
      out[tab] = buildNetworkView(events, {
        tab,
        include: netInclude,
        exclude: netExclude,
        sort: netSort,
        status: netStatus,
        method: netMethod,
      }).length;
    }
    return out;
  }, [events, netInclude, netExclude, netSort, netStatus, netMethod]);

  const allowCheck = !readOnly && !!onLookup;

  return (
    <section className="ui-card p-4">
      <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
        <h2 className="text-sm font-medium uppercase tracking-wider text-[var(--color-fog)]">
          {t("viewer.network")}
        </h2>
        <div className="flex flex-wrap items-center gap-2">
          <p className="font-mono text-xs text-[var(--color-fog)]">
            {t("viewer.networkTotal")}: {events.length}
          </p>
          <button
            type="button"
            disabled={readOnly || enrichBusy || events.length === 0 || !onEnrich}
            onClick={() => onEnrich?.()}
            className="rounded border border-[var(--color-signal)]/40 px-2 py-1 text-xs text-[var(--color-signal-2)] hover:bg-[var(--color-signal-dim)] disabled:opacity-40"
          >
            {enrichBusy ? t("viewer.tiEnriching") : t("viewer.tiEnrich")}
          </button>
          <button
            type="button"
            disabled={readOnly || clearBusy || filtered.length === 0 || !onClearTab}
            onClick={() => onClearTab?.(netTab)}
            className="rounded border border-[var(--color-line)] px-2 py-1 text-xs text-[var(--color-fog)] hover:text-[var(--color-snow)] disabled:opacity-40"
          >
            {t("viewer.clearTab")}
          </button>
          <button
            type="button"
            disabled={readOnly || clearBusy || events.length === 0 || !onClearAll}
            onClick={() => onClearAll?.()}
            className="rounded border border-[var(--color-danger)]/40 px-2 py-1 text-xs text-[var(--color-danger)] disabled:opacity-40"
          >
            {t("viewer.clearAll")}
          </button>
        </div>
      </div>

      <div className="mb-3 flex flex-wrap gap-1 border-b border-[var(--color-line)] pb-2">
        {(
          [
            ["all", t("viewer.tabAll")],
            ["dns", t("viewer.tabDns")],
            ["http", t("viewer.tabHttp")],
            ["url", t("viewer.tabUrl")],
            ["fqdn", t("viewer.tabFqdn")],
            ["ip", t("viewer.tabIp")],
            ["ti", t("viewer.tabTi")],
            ["tree", t("viewer.tabTree")],
            ["netflow", t("viewer.tabNetflow")],
          ] as const
        ).map(([key, label]) => (
          <button
            key={key}
            type="button"
            onClick={() => setNetTab(key)}
            className={[
              "rounded-md px-2.5 py-1.5 text-xs transition",
              netTab === key
                ? "bg-[var(--color-panel-2)] text-[var(--color-signal-2)]"
                : "text-[var(--color-fog)] hover:text-[var(--color-snow)]",
            ].join(" ")}
          >
            {label}
            <span className="ml-1 font-mono opacity-70">{tabCounts[key]}</span>
          </button>
        ))}
      </div>

      <div className="mb-3 grid gap-2 rounded-lg border border-[var(--color-line)]/70 bg-[var(--color-ink)]/30 p-3 md:grid-cols-2 lg:grid-cols-3">
        <label className="block space-y-1 text-xs">
          <span className="text-[var(--color-fog)]">{t("viewer.filterInclude")}</span>
          <input
            value={netInclude}
            onChange={(e) => setNetInclude(e.target.value)}
            placeholder={t("viewer.filterIncludePh")}
            className="w-full rounded-md border border-[var(--color-line)] bg-[var(--color-ink)] px-2 py-1.5 font-mono text-xs text-[var(--color-snow)]"
          />
        </label>
        <label className="block space-y-1 text-xs">
          <span className="text-[var(--color-fog)]">{t("viewer.filterExclude")}</span>
          <input
            value={netExclude}
            onChange={(e) => setNetExclude(e.target.value)}
            placeholder={t("viewer.filterExcludePh")}
            className="w-full rounded-md border border-[var(--color-line)] bg-[var(--color-ink)] px-2 py-1.5 font-mono text-xs text-[var(--color-snow)]"
          />
        </label>
        <label className="block space-y-1 text-xs">
          <span className="text-[var(--color-fog)]">{t("viewer.filterSort")}</span>
          <select
            value={netSort}
            onChange={(e) => setNetSort(e.target.value as NetSort)}
            className="w-full rounded-md border border-[var(--color-line)] bg-[var(--color-ink)] px-2 py-1.5 text-xs text-[var(--color-snow)]"
          >
            <option value="time_desc">{t("viewer.sortTimeDesc")}</option>
            <option value="time_asc">{t("viewer.sortTimeAsc")}</option>
            <option value="host_asc">{t("viewer.sortHostAsc")}</option>
            <option value="host_desc">{t("viewer.sortHostDesc")}</option>
            <option value="status_asc">{t("viewer.sortStatusAsc")}</option>
            <option value="status_desc">{t("viewer.sortStatusDesc")}</option>
            <option value="method_asc">{t("viewer.sortMethod")}</option>
            <option value="url_asc">{t("viewer.sortUrl")}</option>
          </select>
        </label>
        <label className="block space-y-1 text-xs">
          <span className="text-[var(--color-fog)]">{t("viewer.filterStatus")}</span>
          <select
            value={netStatus}
            onChange={(e) => setNetStatus(e.target.value as StatusFilter)}
            className="w-full rounded-md border border-[var(--color-line)] bg-[var(--color-ink)] px-2 py-1.5 text-xs text-[var(--color-snow)]"
          >
            <option value="all">{t("viewer.statusAll")}</option>
            <option value="2xx">2xx</option>
            <option value="3xx">3xx</option>
            <option value="4xx">4xx</option>
            <option value="5xx">5xx</option>
            <option value="error">{t("viewer.statusError")}</option>
            <option value="dns_only">{t("viewer.statusDnsOnly")}</option>
          </select>
        </label>
        <label className="block space-y-1 text-xs">
          <span className="text-[var(--color-fog)]">{t("viewer.filterMethod")}</span>
          <select
            value={netMethod}
            onChange={(e) => setNetMethod(e.target.value)}
            className="w-full rounded-md border border-[var(--color-line)] bg-[var(--color-ink)] px-2 py-1.5 text-xs text-[var(--color-snow)]"
          >
            <option value="all">{t("viewer.methodAll")}</option>
            <option value="GET">GET</option>
            <option value="POST">POST</option>
            <option value="PUT">PUT</option>
            <option value="PATCH">PATCH</option>
            <option value="DELETE">DELETE</option>
            <option value="HEAD">HEAD</option>
            <option value="OPTIONS">OPTIONS</option>
          </select>
        </label>
        <div className="flex items-end">
          <button
            type="button"
            onClick={() => {
              setNetInclude("");
              setNetExclude("");
              setNetSort("time_desc");
              setNetStatus("all");
              setNetMethod("all");
            }}
            className="w-full rounded-md border border-[var(--color-line)] px-2 py-1.5 text-xs text-[var(--color-fog)] hover:text-[var(--color-snow)]"
          >
            {t("viewer.resetFilters")}
          </button>
        </div>
      </div>

      <p className="mb-2 font-mono text-[10px] text-[var(--color-fog)]">
        {netTab === "tree"
          ? `${t("viewer.showing")}: ${requestTree.length} ${t("viewer.treePages")} · ${filtered.length} HTTP`
          : `${t("viewer.showing")}: ${filtered.length} / ${events.length}`}
      </p>

      <div className="max-h-[420px] overflow-auto">
        {filtered.length === 0 ? (
          <div className="rounded-lg border border-dashed border-[var(--color-line)] px-3 py-8 text-center text-sm text-[var(--color-fog)]">
            {events.length === 0 ? t("viewer.networkEmpty") : t("viewer.networkFilteredEmpty")}
          </div>
        ) : netTab === "tree" ? (
          <RequestTreeView
            pages={requestTree}
            expanded={treeExpanded}
            setExpanded={setTreeExpanded}
            tiMap={tiMap}
            lookupBusy={lookupBusy}
            onLookup={allowCheck ? onLookup! : undefined}
          />
        ) : netTab === "netflow" ? (
          <table className="w-full text-left text-xs">
            <thead className="sticky top-0 bg-[var(--color-panel)] text-[var(--color-fog)]">
              <tr>
                <th className="px-2 py-2 font-medium">{t("viewer.colTime")}</th>
                <th className="px-2 py-2 font-medium">{t("viewer.colProto")}</th>
                <th className="px-2 py-2 font-medium">{t("viewer.colDst")}</th>
                <th className="px-2 py-2 font-medium">{t("viewer.colStatus")}</th>
                <th className="px-2 py-2 font-medium">{t("viewer.colDetail")}</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((ev, idx) => {
                const host = hostOf(ev.url || ev.query);
                const ind = primaryIndicator(ev, "netflow");
                const ti = resolveTI(tiMap, ind, ev);
                const busyKey = ind ? tiKey(ind.value, ind.kind) : "";
                const proto = (ev.url || "").startsWith("https")
                  ? "HTTPS"
                  : (ev.url || "").startsWith("http")
                    ? "HTTP"
                    : ev.type?.toUpperCase() || "—";
                return (
                  <tr key={`${ev.id || idx}-flow`} className="border-t border-[var(--color-line)]/60">
                    <td className="whitespace-nowrap px-2 py-2 font-mono text-[var(--color-fog)]">
                      {ev.ts ? new Date(ev.ts).toLocaleTimeString() : "—"}
                    </td>
                    <td className="px-2 py-2 font-mono text-[var(--color-signal-2)]">{proto}</td>
                    <td className="px-2 py-2 font-mono">{host || "—"}</td>
                    <td className="px-2 py-2 font-mono">{ev.status ?? "—"}</td>
                    <td className="max-w-[420px] px-2 py-2">
                      <div className="flex items-center gap-2">
                        <span className="min-w-0 truncate">{ev.url || ev.query || "—"}</span>
                        <TICheck
                          ti={ti}
                          busy={lookupBusy === busyKey}
                          onCheck={
                            allowCheck && ind
                              ? () => onLookup?.(ind.value, ind.kind)
                              : undefined
                          }
                        />
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        ) : (
          <div className="space-y-2">
            {filtered.map((ev, idx) => {
              const ind = primaryIndicator(ev, netTab);
              const ti = resolveTI(tiMap, ind, ev);
              const busyKey = ind ? tiKey(ind.value, ind.kind) : "";
              return (
                <NetworkRow
                  key={`${ev.id || ev.ts || idx}-${ev.url || ev.query || ev.indicator || idx}`}
                  ev={ev}
                  tab={netTab}
                  ti={ti}
                  lookupBusy={lookupBusy === busyKey}
                  onLookup={
                    allowCheck && ind ? () => onLookup?.(ind.value, ind.kind) : undefined
                  }
                />
              );
            })}
          </div>
        )}
      </div>
    </section>
  );
}

export function normalizeHistoryNetworkEvents(raw: Array<Record<string, unknown>>): NetworkEventItem[] {
  return raw.map((n) => {
    const createdAt = n.createdAt != null ? String(n.createdAt) : undefined;
    const ts = n.ts != null ? String(n.ts) : createdAt;
    return {
      ...(n as unknown as NetworkEventItem),
      id: n.id != null ? String(n.id) : undefined,
      type: String(n.type || "http"),
      ts,
    };
  });
}
