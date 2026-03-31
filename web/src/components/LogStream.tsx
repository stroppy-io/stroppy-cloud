import { useEffect, useRef, useState, useMemo } from "react";
import { WSConnection } from "@/api/ws";
import { getRunLogs } from "@/api/client";
import type { WSMessage } from "@/api/types";
import { ArrowDown, Filter } from "lucide-react";

interface AgentLogLine {
  command_id?: string;
  machine_id?: string;
  line: string;
  stream?: string;
}

interface DisplayLine {
  machineID: string;
  text: string;
  ts: number;
}

interface LogStreamProps {
  runID?: string;
}

const MACHINE_COLORS = [
  "text-cyan-400",
  "text-yellow-400",
  "text-green-400",
  "text-pink-400",
  "text-orange-400",
  "text-violet-400",
  "text-blue-400",
  "text-rose-400",
];

function machineColor(id: string, colorMap: Map<string, string>): string {
  if (!colorMap.has(id)) {
    colorMap.set(id, MACHINE_COLORS[colorMap.size % MACHINE_COLORS.length]);
  }
  return colorMap.get(id)!;
}

export function LogStream({ runID }: LogStreamProps) {
  const [lines, setLines] = useState<DisplayLine[]>([]);
  const [autoScroll, setAutoScroll] = useState(true);
  const [selectedSource, setSelectedSource] = useState<string | null>(null); // null = all
  const containerRef = useRef<HTMLDivElement>(null);
  const colorMapRef = useRef(new Map<string, string>());

  // Collect unique sources.
  const sources = useMemo(() => {
    const s = new Set<string>();
    for (const l of lines) s.add(l.machineID);
    return Array.from(s).sort();
  }, [lines]);

  // Filtered lines.
  const filteredLines = useMemo(
    () => selectedSource ? lines.filter((l) => l.machineID === selectedSource) : lines,
    [lines, selectedSource]
  );

  // Load historical logs from VictoriaLogs on mount.
  useEffect(() => {
    if (!runID) return;
    getRunLogs(runID)
      .then((rawLines) => {
        const historical: DisplayLine[] = rawLines.map((raw) => {
          try {
            const obj = JSON.parse(raw);
            return { machineID: obj.machine_id || "system", text: obj.line || raw, ts: 0 };
          } catch {
            return { machineID: "system", text: raw, ts: 0 };
          }
        });
        if (historical.length > 0) {
          setLines((prev) => [...historical, ...prev]);
        }
      })
      .catch(() => {});
  }, [runID]);

  // Live WebSocket stream.
  useEffect(() => {
    const ws = new WSConnection(runID);
    const unsub = ws.onMessage((msg: WSMessage) => {
      if (msg.type === "agent_log") {
        const payload = msg.payload as AgentLogLine;
        setLines((prev) => {
          const next = [...prev, { machineID: payload.machine_id || "unknown", text: payload.line, ts: Date.now() }];
          return next.length > 5000 ? next.slice(-5000) : next;
        });
      } else if (msg.type === "log") {
        const p = msg.payload as { message?: string };
        if (p.message) {
          setLines((prev) => {
            const next = [...prev, { machineID: msg.node_id || "system", text: p.message!, ts: Date.now() }];
            return next.length > 5000 ? next.slice(-5000) : next;
          });
        }
      }
    });
    ws.connect();
    return () => { unsub(); ws.disconnect(); };
  }, [runID]);

  // Auto-scroll.
  useEffect(() => {
    if (autoScroll && containerRef.current) {
      containerRef.current.scrollTop = containerRef.current.scrollHeight;
    }
  }, [filteredLines, autoScroll]);

  function handleScroll() {
    if (!containerRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } = containerRef.current;
    const atBottom = scrollHeight - scrollTop - clientHeight < 40;
    if (atBottom !== autoScroll) setAutoScroll(atBottom);
  }

  return (
    <div className="flex flex-col h-full relative">
      {/* Toolbar */}
      <div className="flex items-center gap-3 px-3 py-1.5 border-b border-border bg-[#060606]">
        {/* Source filter */}
        <div className="flex items-center gap-1.5">
          <Filter className="h-3 w-3 text-zinc-600" />
          <button
            onClick={() => setSelectedSource(null)}
            className={`px-2 py-0.5 text-[10px] font-mono border transition-colors ${
              selectedSource === null
                ? "border-primary/50 text-primary bg-primary/5"
                : "border-zinc-800 text-zinc-600 hover:text-zinc-400"
            }`}
          >
            all
          </button>
          {sources.map((src) => {
            const color = machineColor(src, colorMapRef.current);
            return (
              <button
                key={src}
                onClick={() => setSelectedSource(selectedSource === src ? null : src)}
                className={`px-2 py-0.5 text-[10px] font-mono border transition-colors ${
                  selectedSource === src
                    ? "border-primary/50 text-primary bg-primary/5"
                    : `border-zinc-800 ${color} hover:bg-zinc-900`
                }`}
              >
                {src.replace(/^stroppy-agent-/, "").replace(/^run-\d+-/, "")}
              </button>
            );
          })}
        </div>

        <div className="flex-1" />

        {/* Line count */}
        <span className="text-[10px] text-zinc-600 font-mono">
          {filteredLines.length}{selectedSource ? `/${lines.length}` : ""} lines
        </span>

        {/* Auto-scroll toggle */}
        <button
          onClick={() => {
            const next = !autoScroll;
            setAutoScroll(next);
            if (next && containerRef.current) {
              containerRef.current.scrollTop = containerRef.current.scrollHeight;
            }
          }}
          className={`flex items-center gap-1 px-2 py-0.5 text-[10px] font-mono border transition-colors ${
            autoScroll
              ? "border-emerald-800 text-emerald-400 bg-emerald-500/5"
              : "border-zinc-800 text-zinc-600 hover:text-zinc-400"
          }`}
          title={autoScroll ? "Auto-scroll ON — click to pause" : "Auto-scroll OFF — click to resume"}
        >
          <ArrowDown className={`h-3 w-3 ${autoScroll ? "animate-bounce" : ""}`} />
          {autoScroll ? "live" : "paused"}
        </button>
      </div>

      {/* Log output */}
      <div
        ref={containerRef}
        onScroll={handleScroll}
        className="flex-1 overflow-auto p-2 font-mono text-xs leading-5 bg-[#0a0a0a] text-gray-200"
      >
        {filteredLines.length === 0 ? (
          <span className="text-zinc-600">
            {lines.length === 0 ? "Waiting for agent output..." : "No logs matching filter."}
          </span>
        ) : (
          filteredLines.map((dl, i) => {
            const color = machineColor(dl.machineID, colorMapRef.current);
            return (
              <div key={i} className="flex hover:bg-white/5">
                <span
                  className={`${color} shrink-0 w-32 truncate select-none pr-2 text-right cursor-pointer hover:underline`}
                  onClick={() => setSelectedSource(selectedSource === dl.machineID ? null : dl.machineID)}
                  title={`Filter by ${dl.machineID}`}
                >
                  [{dl.machineID.replace(/^stroppy-agent-/, "").replace(/^run-\d+-/, "")}]
                </span>
                <span className="whitespace-pre-wrap break-all">{dl.text}</span>
              </div>
            );
          })
        )}
      </div>
    </div>
  );
}
