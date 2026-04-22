import { useEffect, useRef, useState, useMemo, useCallback } from "react";
import { VList, type VListHandle } from "virtua";
import { WSConnection } from "@/api/ws";
import { getRunLogs } from "@/api/client";
import type { WSMessage, Snapshot } from "@/api/types";
import { ArrowDown, Server, Zap, Search, X, WrapText, AlignLeft, Check } from "lucide-react";
import { MultiFilter, type FilterOption } from "@/components/ui/multi-filter";

/* ---------- types ---------- */

interface AgentLogLine {
  command_id?: string;
  machine_id?: string;
  action?: string;
  line: string;
  stream?: string;
}

interface DisplayLine {
  machineID: string;
  action: string;
  text: string;
  ts: number;
}

/* ---------- constants ---------- */

const MAX_LINES = 50_000;
const PAGE_SIZE = 500;
const PREFETCH_PX = 50;

const MACHINE_COLORS = [
  "text-cyan-400", "text-yellow-400", "text-green-400", "text-pink-400",
  "text-orange-400", "text-violet-400", "text-blue-400", "text-rose-400",
];

const ACTION_LABELS: Record<string, string> = {
  install_postgres: "Install PostgreSQL", config_postgres: "Configure PostgreSQL",
  install_mysql: "Install MySQL", config_mysql: "Configure MySQL",
  install_picodata: "Install Picodata", config_picodata: "Configure Picodata",
  install_ydb: "Install YDB", config_ydb: "Configure YDB Static",
  init_ydb: "Init YDB Cluster", start_ydb_db: "Start YDB Database",
  init_ydb_cluster: "Init YDB Cluster", start_ydb_database: "Start YDB Database",
  install_monitor: "Install Monitoring", config_monitor: "Configure Monitoring",
  install_stroppy: "Install Stroppy", run_stroppy: "Run Stroppy",
  install_etcd: "Install etcd", config_etcd: "Configure etcd",
  install_patroni: "Install Patroni", config_patroni: "Configure Patroni",
  install_pgbouncer: "Install PgBouncer", config_pgbouncer: "Configure PgBouncer",
  install_haproxy: "Install HAProxy", config_haproxy: "Configure HAProxy",
  install_proxysql: "Install ProxySQL", config_proxysql: "Configure ProxySQL",
  shutdown: "Shutdown", network: "Network", machines: "Machines",
  install_db: "Install DB", configure_db: "Configure DB",
  configure_monitor: "Configure Monitoring",
  install_proxy: "Install Proxy", configure_proxy: "Configure Proxy",
  configure_etcd: "Configure etcd", configure_patroni: "Configure Patroni",
  configure_pgbouncer: "Configure PgBouncer", teardown: "Teardown",
};

function phaseLabel(p: string): string {
  return !p ? "Other" : ACTION_LABELS[p] || p.replace(/_/g, " ").replace(/\b\w/g, c => c.toUpperCase());
}

/* ---------- phase resolution ---------- */

const PHASE_ACTIONS: Record<string, string[]> = {
  install_db: ["install_postgres", "install_mysql", "install_picodata", "install_ydb"],
  configure_db: ["config_postgres", "config_mysql", "config_picodata", "config_ydb"],
  install_monitor: ["install_monitor"], configure_monitor: ["config_monitor"],
  install_stroppy: ["install_stroppy"], run_stroppy: ["run_stroppy"],
  install_etcd: ["install_etcd"], configure_etcd: ["config_etcd"],
  install_patroni: ["install_patroni"], configure_patroni: ["config_patroni"],
  install_pgbouncer: ["install_pgbouncer"], configure_pgbouncer: ["config_pgbouncer"],
  install_proxy: ["install_haproxy", "install_proxysql"],
  configure_proxy: ["config_haproxy", "config_proxysql"],
  init_ydb_cluster: ["init_ydb"],
  start_ydb_database: ["start_ydb_db"],
};

function buildA2P(phases: string[]): Record<string, string> {
  const m: Record<string, string> = {};
  for (const ph of phases) {
    const aa = PHASE_ACTIONS[ph];
    if (aa) for (const a of aa) m[a] = ph;
    m[ph] = ph;
  }
  return m;
}

function resolvePhase(action: string, a2p: Record<string, string>): string {
  return a2p[action] || action || "";
}

function expandPhasesToActions(phases: Set<string>): string[] {
  const actions = new Set<string>();
  for (const p of phases) {
    actions.add(p);
    const aa = PHASE_ACTIONS[p];
    if (aa) for (const a of aa) actions.add(a);
  }
  return [...actions];
}

/* ---------- helpers ---------- */

function machineColor(id: string, cm: Map<string, string>): string {
  if (!cm.has(id)) cm.set(id, MACHINE_COLORS[cm.size % MACHINE_COLORS.length]);
  return cm.get(id)!;
}

function shortMachine(id: string): string {
  return id.replace(/^stroppy-agent-/, "").replace(/^run-[a-z0-9]+-/, "");
}

function parseLine(raw: string): DisplayLine {
  try {
    const o = JSON.parse(raw);
    return { machineID: o.machine_id || "server", action: o.action || o.node_id || "", text: o._msg || o.line || raw, ts: o._time ? new Date(o._time).getTime() : 0 };
  } catch { return { machineID: "server", action: "", text: raw, ts: 0 }; }
}

function extractScopes(snap: Snapshot | null | undefined) {
  const machines: string[] = ["server"];
  const phases: string[] = [];
  if (snap) {
    const targets = snap.state?.targets;
    if (Array.isArray(targets)) for (const t of targets) if (t.id && !machines.includes(t.id)) machines.push(t.id);
    for (const n of snap.nodes) phases.push(n.id);
  }
  return { machines, phases, a2p: buildA2P(phases) };
}

/* ---------- highlight match ---------- */

function HighlightText({ text, search }: { text: string; search: string }) {
  if (!search) return <>{text}</>;
  const idx = text.toLowerCase().indexOf(search.toLowerCase());
  if (idx === -1) return <>{text}</>;
  return (
    <>
      {text.slice(0, idx)}
      <mark className="bg-yellow-500/30 text-yellow-200 rounded-sm px-px">{text.slice(idx, idx + search.length)}</mark>
      {text.slice(idx + search.length)}
    </>
  );
}

/* ---------- component ---------- */

interface LogStreamProps { runID?: string; snapshot?: Snapshot | null; focusPhase?: string | null; }

export function LogStream({ runID, snapshot, focusPhase }: LogStreamProps) {
  const [lines, setLines] = useState<DisplayLine[]>([]);
  const [autoScroll, setAutoScroll] = useState(true);
  const [filterMachines, setFilterMachines] = useState<Set<string>>(new Set());
  const [filterPhases, setFilterPhases] = useState<Set<string>>(new Set());
  const [searchInput, setSearchInput] = useState("");
  const [searchQuery, setSearchQuery] = useState("");
  const [searchResults, setSearchResults] = useState<DisplayLine[] | null>(null);
  const [searchLoading, setSearchLoading] = useState(false);
  const [wrapLines, setWrapLines] = useState(true);
  const [copiedLine, setCopiedLine] = useState<number | null>(null);
  const colorMapRef = useRef(new Map<string, string>());
  const vlistRef = useRef<VListHandle>(null);
  const [logError, setLogError] = useState<string | null>(null);
  const [loadingOlder, setLoadingOlder] = useState(false);
  const noMoreOlderRef = useRef(false);
  const [shifting, setShifting] = useState(false);
  const loadingOlderRef = useRef(false);

  const scopes = useMemo(() => extractScopes(snapshot), [snapshot]);
  const a2p = scopes.a2p;

  // --- Focus on phase (from "View in Logs" click) ---
  useEffect(() => {
    if (focusPhase) {
      setFilterPhases(new Set([focusPhase]));
      setAutoScroll(false); // don't jump to bottom, show phase from the start
    }
  }, [focusPhase]);

  // --- Refetch when phase filter changes — gets older logs for filtered phase ---
  useEffect(() => {
    if (!runID || filterPhases.size === 0) return;
    const actions = expandPhasesToActions(filterPhases);
    getRunLogs(runID, { actions, desc: false, limit: 2000 })
      .then((raw) => {
        const parsed = raw.map(parseLine);
        setLines((prev) => {
          const seen = new Set(parsed.map((l) => `${l.ts}|${l.text}`));
          const streamed = prev.filter((l) => !seen.has(`${l.ts}|${l.text}`));
          return [...parsed, ...streamed];
        });
      })
      .catch(() => { /* silent — keep existing lines */ });
  }, [runID, filterPhases]);

  // --- Line highlight ---
  // Read target line from URL hash once on mount (for initial load strategy + scroll).
  const [initialTargetLine] = useState<number | null>(() => {
    const hash = window.location.hash;
    if (hash.startsWith("#L")) {
      const n = parseInt(hash.slice(2));
      return isNaN(n) ? null : n;
    }
    return null;
  });
  // Visual highlight — updated by click or URL, no scroll side effect.
  const [highlightLine, setHighlightLine] = useState<number | null>(initialTargetLine);

  // --- Debounce search ---
  const searchTimer = useRef<ReturnType<typeof setTimeout>>(undefined);
  useEffect(() => {
    clearTimeout(searchTimer.current);
    if (!searchInput.trim()) { setSearchQuery(""); setSearchResults(null); return; }
    searchTimer.current = setTimeout(() => setSearchQuery(searchInput.trim()), 300);
    return () => clearTimeout(searchTimer.current);
  }, [searchInput]);

  // --- Server-side search ---
  useEffect(() => {
    if (!searchQuery || !runID) { setSearchResults(null); return; }
    setSearchLoading(true);
    getRunLogs(runID, { search: searchQuery, limit: 200 })
      .then((raw) => setSearchResults(raw.map(parseLine)))
      .finally(() => setSearchLoading(false));
  }, [searchQuery, runID]);

  // --- Append line (live) ---
  const appendLine = (dl: DisplayLine) => {
    setLines((prev) => {
      const next = [...prev, dl];
      return next.length > MAX_LINES ? next.slice(next.length - MAX_LINES) : next;
    });
  };

  // --- Cross-filtered counts ---
  const { machineOptions, phaseOptions } = useMemo(() => {
    const hasMF = filterMachines.size > 0, hasPF = filterPhases.size > 0;
    const mc: Record<string, number> = {}, pc: Record<string, number> = {};
    for (const l of lines) {
      const ph = resolvePhase(l.action, a2p);
      if (!hasPF || filterPhases.has(ph)) mc[l.machineID] = (mc[l.machineID] || 0) + 1;
      if ((!hasMF || filterMachines.has(l.machineID)) && ph) pc[ph] = (pc[ph] || 0) + 1;
    }
    return {
      machineOptions: scopes.machines.map((m): FilterOption => ({ value: m, label: shortMachine(m), count: mc[m] || 0, color: machineColor(m, colorMapRef.current) })),
      phaseOptions: scopes.phases.map((ph): FilterOption => ({ value: ph, label: phaseLabel(ph), count: pc[ph] || 0 })),
    };
  }, [lines, scopes, filterMachines, filterPhases, a2p]);

  // --- Filtered rows ---
  const rows = useMemo(() => {
    const src = searchResults ?? lines;
    const hasMF = filterMachines.size > 0, hasPF = filterPhases.size > 0;
    if (!hasMF && !hasPF) return src;
    return src.filter((l) => {
      if (hasMF && !filterMachines.has(l.machineID)) return false;
      if (hasPF && !filterPhases.has(resolvePhase(l.action, a2p))) return false;
      return true;
    });
  }, [lines, searchResults, filterMachines, filterPhases, a2p]);

  // --- Initial load ---
  const [initialLoadDone, setInitialLoadDone] = useState(false);
  useEffect(() => {
    if (!runID) return;
    noMoreOlderRef.current = false;
    setInitialLoadDone(false);

    // Merge historical logs with any WS-streamed lines that arrived first.
    // Dedupe by (ts, text) — WS-streamed lines have ts=Date.now(), HTTP lines have _time from VL.
    const merge = (prev: DisplayLine[], historical: DisplayLine[]): DisplayLine[] => {
      if (prev.length === 0) return historical;
      const seen = new Set(historical.map((l) => `${l.ts}|${l.text}`));
      const streamed = prev.filter((l) => !seen.has(`${l.ts}|${l.text}`));
      return [...historical, ...streamed];
    };

    if (initialTargetLine !== null) {
      const loadCount = initialTargetLine + 200;
      getRunLogs(runID, { desc: false, limit: loadCount })
        .then((raw) => {
          const parsed = raw.map(parseLine);
          if (parsed.length < loadCount) noMoreOlderRef.current = true;
          setLines((prev) => merge(prev, parsed));
          setLogError(null);
        })
        .catch((err) => setLogError(err instanceof Error ? err.message : "Failed to load logs"))
        .finally(() => setInitialLoadDone(true));
    } else {
      getRunLogs(runID, { desc: true, limit: PAGE_SIZE })
        .then((raw) => {
          const parsed = raw.map(parseLine).reverse();
          if (parsed.length < PAGE_SIZE) noMoreOlderRef.current = true;
          setLines((prev) => merge(prev, parsed));
          setLogError(null);
        })
        .catch((err) => setLogError(err instanceof Error ? err.message : "Failed to load logs"))
        .finally(() => setInitialLoadDone(true));
    }
  }, [runID, initialTargetLine]);

  // Scroll to target after initial render (once). Double-rAF to ensure VList has measured all items.
  const didInitialScroll = useRef(false);
  useEffect(() => {
    if (!didInitialScroll.current && lines.length > 0 && vlistRef.current) {
      didInitialScroll.current = true;
      const doScroll = () => {
        if (!vlistRef.current) return;
        if (initialTargetLine !== null && initialTargetLine <= lines.length) {
          vlistRef.current.scrollToIndex(initialTargetLine - 1, { align: "center" });
          setAutoScroll(false);
        } else {
          vlistRef.current.scrollToIndex(lines.length - 1, { align: "end" });
        }
      };
      // Double rAF: first lets React commit, second lets VList measure.
      requestAnimationFrame(() => requestAnimationFrame(doScroll));
    }
  }, [lines.length, initialTargetLine]);

  // --- WebSocket ---
  useEffect(() => {
    const ws = new WSConnection(runID);
    const unsub = ws.onMessage((msg: WSMessage) => {
      if (msg.type === "agent_log") {
        const p = msg.payload as AgentLogLine;
        appendLine({ machineID: p.machine_id || "unknown", action: p.action || "", text: p.line, ts: Date.now() });
      } else if (msg.type === "log") {
        const p = msg.payload as Record<string, unknown>;
        if (p.message) {
          const skip = new Set(["level", "message", "time", "node_id"]);
          const extras = Object.entries(p).filter(([k]) => !skip.has(k)).map(([k, v]) => `${k}=${v}`).join("  ");
          appendLine({ machineID: "server", action: String(msg.node_id || ""), text: extras ? `${p.message}  ${extras}` : String(p.message), ts: Date.now() });
        }
      }
    });
    ws.connect();
    return () => { unsub(); ws.disconnect(); };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [runID]);

  // --- Programmatic scroll guard ---
  const progScrollRef = useRef(false);
  function scrollToBottom() {
    if (vlistRef.current && rows.length > 0) {
      progScrollRef.current = true;
      vlistRef.current.scrollToIndex(rows.length - 1, { align: "end" });
    }
  }

  useEffect(() => {
    if (autoScroll && !searchResults) scrollToBottom();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [lines.length, rows.length]);

  // --- Load older ---
  function loadOlderLogs() {
    if (loadingOlderRef.current || noMoreOlderRef.current || !runID || searchResults) return;
    const oldest = lines.find((l) => l.ts > 0);
    if (!oldest) return;
    loadingOlderRef.current = true;
    setLoadingOlder(true);
    setShifting(true);
    getRunLogs(runID, { end: new Date(oldest.ts - 1).toISOString(), desc: true, limit: PAGE_SIZE })
      .then((raw) => {
        const older = raw.map(parseLine).reverse();
        if (older.length < PAGE_SIZE) noMoreOlderRef.current = true;
        if (older.length > 0) {
          setLines((prev) => {
            const merged = [...older, ...prev];
            return merged.length > MAX_LINES ? merged.slice(merged.length - MAX_LINES) : merged;
          });
        }
      })
      .finally(() => {
        loadingOlderRef.current = false;
        setLoadingOlder(false);
        requestAnimationFrame(() => setShifting(false));
      });
  }

  const handleScroll = (offset: number) => {
    if (!vlistRef.current) return;
    const { scrollSize, viewportSize } = vlistRef.current;
    setAutoScroll(scrollSize - offset - viewportSize < 40);
    if (!progScrollRef.current && offset < PREFETCH_PX) loadOlderLogs();
    if (progScrollRef.current && offset > PREFETCH_PX) progScrollRef.current = false;
  };
  const handleScrollEnd = () => { progScrollRef.current = false; };

  // --- Copy line link ---
  const copyLineLink = useCallback((lineNum: number) => {
    const url = `${window.location.origin}/runs/${runID}?tab=logs#L${lineNum}`;
    navigator.clipboard.writeText(url).then(() => {
      setCopiedLine(lineNum);
      // Update URL hash without reload.
      window.history.replaceState(null, "", `#L${lineNum}`);
      setHighlightLine(lineNum);
      setTimeout(() => setCopiedLine(null), 1500);
    });
  }, [runID]);

  const isSearching = searchResults !== null;
  const hasFilters = filterMachines.size > 0 || filterPhases.size > 0;

  return (
    <div className="flex flex-col h-full relative">
      {/* Toolbar */}
      <div className="flex items-center gap-2 px-3 py-1.5 border-b border-border bg-[#060606]">
        <MultiFilter icon={<Server className="h-3 w-3" />} label="Machine" options={machineOptions} selected={filterMachines} onChange={setFilterMachines} />
        <MultiFilter icon={<Zap className="h-3 w-3" />} label="Phase" options={phaseOptions} selected={filterPhases} onChange={setFilterPhases} />

        {/* Search */}
        <div className="flex items-center gap-1 px-2 py-0.5 border border-zinc-800 rounded text-[11px] font-mono focus-within:border-zinc-600 transition-colors">
          <Search className="h-3 w-3 text-zinc-600 shrink-0" />
          <input
            type="text"
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            placeholder="Search logs..."
            className="bg-transparent outline-none text-zinc-300 placeholder:text-zinc-700 w-28 focus:w-40 transition-all"
          />
          {searchInput && (
            <X className="h-3 w-3 text-zinc-600 hover:text-zinc-300 cursor-pointer shrink-0" onClick={() => setSearchInput("")} />
          )}
        </div>

        <div className="flex-1" />

        <span className="text-[10px] text-zinc-600 font-mono shrink-0">
          {isSearching ? `${rows.length} found` : (hasFilters ? `${rows.length.toLocaleString()}/` : "") + lines.length.toLocaleString()}
          {searchLoading && " ..."}
        </span>

        {/* Wrap toggle */}
        <button
          onClick={() => setWrapLines(!wrapLines)}
          className={`p-1 border transition-colors shrink-0 ${
            wrapLines ? "border-primary/40 text-primary bg-primary/5" : "border-zinc-800 text-zinc-600 hover:text-zinc-400"
          }`}
          title={wrapLines ? "Wrap: ON (click for no-wrap)" : "Wrap: OFF (click to wrap)"}
        >
          {wrapLines ? <WrapText className="h-3 w-3" /> : <AlignLeft className="h-3 w-3" />}
        </button>

        <button
          onClick={() => { scrollToBottom(); setAutoScroll(true); }}
          className={`flex items-center gap-1 px-2 py-0.5 text-[10px] font-mono border transition-colors shrink-0 ${
            autoScroll ? "border-emerald-800 text-emerald-400 bg-emerald-500/5" : "border-zinc-800 text-zinc-600 hover:text-zinc-400"
          }`}
        >
          <ArrowDown className={`h-3 w-3 ${autoScroll ? "animate-bounce" : ""}`} />
          {autoScroll ? "live" : "paused"}
        </button>
      </div>

      {/* Log output */}
      <div className="flex-1 min-h-0 overflow-hidden bg-[#0a0a0a] text-gray-200 font-mono text-xs flex flex-col">
        {loadingOlder && (
          <div className="shrink-0 py-1 text-center text-[10px] font-mono text-zinc-500 border-b border-zinc-900">
            Loading older logs...
          </div>
        )}

        {logError ? (
          <div className="p-3 text-destructive text-xs">{logError}</div>
        ) : rows.length === 0 ? (
          <div className="p-3 text-zinc-600 flex items-center gap-2">
            {isSearching ? "No matches." :
             lines.length === 0 && !initialLoadDone ? (
               <>
                 <span className="inline-block w-2 h-2 rounded-full bg-zinc-600 animate-pulse" />
                 Loading logs...
               </>
             ) :
             lines.length === 0 ? "No logs yet. Will appear as agents start reporting." :
             "No logs matching filter."}
          </div>
        ) : (
          <VList ref={vlistRef} style={{ flex: 1 }} shift={shifting} onScroll={handleScroll} onScrollEnd={handleScrollEnd}>
            {rows.map((dl, i) => {
              const lineNum = i + 1;
              const color = machineColor(dl.machineID, colorMapRef.current);
              const isHighlighted = highlightLine === lineNum;
              const isCopied = copiedLine === lineNum;
              return (
                <div
                  key={i}
                  className={`flex group hover:bg-white/[0.03] px-1 ${
                    wrapLines ? "py-px" : "h-5"
                  } ${isHighlighted ? "bg-yellow-500/10 border-l-2 border-yellow-500" : ""}`}
                >
                  {/* Line number — blurred until hover, click to copy link */}
                  <span
                    className={`shrink-0 w-10 text-right pr-2 select-none text-[10px] leading-5 cursor-pointer transition-all ${
                      isCopied
                        ? "text-emerald-400"
                        : "text-zinc-800 group-hover:text-zinc-500"
                    }`}
                    onClick={() => copyLineLink(lineNum)}
                    title="Click to copy link to this line"
                  >
                    {isCopied ? <Check className="h-3 w-3 inline" /> : lineNum}
                  </span>
                  {/* Machine label */}
                  <span className={`${color} shrink-0 w-24 truncate select-none pr-1 text-right text-[11px] leading-5`}>
                    [{shortMachine(dl.machineID)}]
                  </span>
                  {/* Log text */}
                  <span
                    className={`text-[11px] leading-5 ${
                      wrapLines
                        ? "whitespace-pre-wrap break-all"
                        : "whitespace-nowrap overflow-hidden text-ellipsis"
                    }`}
                    title={wrapLines ? undefined : dl.text}
                  >
                    <HighlightText text={dl.text} search={searchQuery} />
                  </span>
                </div>
              );
            })}
          </VList>
        )}
      </div>
    </div>
  );
}
