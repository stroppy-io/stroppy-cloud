import { useEffect, useState, useMemo } from "react";
import { Link, useNavigate } from "react-router-dom";
import {
  useReactTable,
  getCoreRowModel,
  getFilteredRowModel,
  getPaginationRowModel,
  getSortedRowModel,
  flexRender,
  type ColumnDef,
  type SortingState,
  type ColumnFiltersState,
  type RowSelectionState,
} from "@tanstack/react-table";
import { listRuns, deleteRun, compareRuns, getGrafanaSettings } from "@/api/client";
import type { RunSummary, ComparisonRow, GrafanaSettings } from "@/api/types";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { MetricsDiff } from "@/components/MetricsDiff";
import {
  RefreshCw,
  Trash2,
  ChevronLeft,
  ChevronRight,
  ChevronsLeft,
  ChevronsRight,
  ArrowUpDown,
  ArrowUp,
  ArrowDown,
  GitCompare,
  X,
  Play,
  AlertCircle,
} from "lucide-react";

// --- Helpers ---

type RunStatus = "done" | "failed" | "running" | "pending";

function deriveStatus(r: RunSummary): RunStatus {
  if (r.failed > 0) return "failed";
  if (r.done === r.total && r.total > 0) return "done";
  if (r.done > 0) return "running";
  return "pending";
}

const STATUS_CONFIG: Record<RunStatus, { label: string; variant: "success" | "destructive" | "warning" | "pending" }> = {
  done: { label: "Done", variant: "success" },
  failed: { label: "Failed", variant: "destructive" },
  running: { label: "Running", variant: "warning" },
  pending: { label: "Pending", variant: "pending" },
};

function formatTimestamp(ts?: string): string {
  if (!ts) return "\u2014";
  const d = new Date(ts);
  if (isNaN(d.getTime())) return "\u2014";
  // Zero-value Go time
  if (d.getFullYear() < 2000) return "\u2014";
  return d.toLocaleString("en-GB", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  });
}

function durationBetween(start?: string, end?: string): string {
  if (!start) return "\u2014";
  const s = new Date(start);
  if (isNaN(s.getTime()) || s.getFullYear() < 2000) return "\u2014";
  const e = end ? new Date(end) : new Date();
  if (isNaN(e.getTime()) || e.getFullYear() < 2000) return "\u2014";
  const sec = Math.max(0, Math.round((e.getTime() - s.getTime()) / 1000));
  if (sec < 60) return `${sec}s`;
  if (sec < 3600) return `${Math.floor(sec / 60)}m ${sec % 60}s`;
  const h = Math.floor(sec / 3600);
  const m = Math.floor((sec % 3600) / 60);
  return `${h}h ${m}m`;
}

// --- Filter button component ---

function FilterChip({
  label,
  active,
  onClick,
}: {
  label: string;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      onClick={onClick}
      className={`px-2.5 py-1 text-[11px] font-mono border transition-colors cursor-pointer ${
        active
          ? "border-primary/60 text-primary bg-primary/8"
          : "border-zinc-800 text-zinc-500 hover:text-zinc-300 hover:border-zinc-700"
      }`}
    >
      {label}
    </button>
  );
}

// --- Column definitions ---

function makeColumns(onDelete: (id: string) => void): ColumnDef<RunSummary>[] {
  return [
    // Checkbox
    {
      id: "select",
      header: ({ table }) => (
        <input
          type="checkbox"
          className="accent-primary w-3.5 h-3.5 cursor-pointer"
          checked={table.getIsAllPageRowsSelected()}
          onChange={table.getToggleAllPageRowsSelectedHandler()}
        />
      ),
      cell: ({ row }) => (
        <input
          type="checkbox"
          className="accent-primary w-3.5 h-3.5 cursor-pointer"
          checked={row.getIsSelected()}
          onChange={row.getToggleSelectedHandler()}
        />
      ),
      enableSorting: false,
      size: 36,
    },
    // ID
    {
      accessorKey: "id",
      header: ({ column }) => (
        <SortableHeader column={column} label="Run ID" />
      ),
      cell: ({ row }) => (
        <Link
          to={`/runs/${row.original.id}`}
          className="font-mono text-xs text-primary hover:underline"
        >
          {row.original.id}
        </Link>
      ),
    },
    // Status
    {
      id: "status",
      accessorFn: (row) => deriveStatus(row),
      header: ({ column }) => (
        <SortableHeader column={column} label="Status" />
      ),
      cell: ({ row }) => {
        const status = deriveStatus(row.original);
        const cfg = STATUS_CONFIG[status];
        return <Badge variant={cfg.variant}>{cfg.label}</Badge>;
      },
      filterFn: (row, _id, value) => {
        if (!value || value === "all") return true;
        return deriveStatus(row.original) === value;
      },
    },
    // DB Kind
    {
      accessorKey: "db_kind",
      header: ({ column }) => (
        <SortableHeader column={column} label="Database" />
      ),
      cell: ({ row }) => (
        <span className="font-mono text-xs text-zinc-400">
          {row.original.db_kind || "\u2014"}
        </span>
      ),
      filterFn: (row, _id, value) => {
        if (!value || value === "all") return true;
        return row.original.db_kind === value;
      },
    },
    // Provider
    {
      accessorKey: "provider",
      header: ({ column }) => (
        <SortableHeader column={column} label="Provider" />
      ),
      cell: ({ row }) => (
        <span className="font-mono text-xs text-zinc-400">
          {row.original.provider || "\u2014"}
        </span>
      ),
      filterFn: (row, _id, value) => {
        if (!value || value === "all") return true;
        return row.original.provider === value;
      },
    },
    // Progress
    {
      id: "progress",
      header: "Progress",
      cell: ({ row }) => {
        const r = row.original;
        const pct = r.total > 0 ? (r.done / r.total) * 100 : 0;
        return (
          <div className="flex items-center gap-2 min-w-[120px]">
            <div className="flex-1 bg-zinc-900 h-1.5 overflow-hidden">
              <div
                className={`h-full transition-all duration-500 ${
                  r.failed > 0 ? "bg-destructive" : "bg-emerald-500"
                }`}
                style={{ width: `${pct}%` }}
              />
            </div>
            <span className="text-[10px] font-mono text-zinc-500 tabular-nums w-12 text-right">
              {r.done}/{r.total}
            </span>
          </div>
        );
      },
      enableSorting: false,
    },
    // Started
    {
      accessorKey: "started_at",
      header: ({ column }) => (
        <SortableHeader column={column} label="Started" />
      ),
      cell: ({ row }) => (
        <span className="text-xs text-zinc-500 font-mono tabular-nums">
          {formatTimestamp(row.original.started_at)}
        </span>
      ),
    },
    // Duration
    {
      id: "duration",
      header: "Duration",
      cell: ({ row }) => (
        <span className="text-xs text-zinc-500 font-mono tabular-nums">
          {durationBetween(row.original.started_at, row.original.finished_at)}
        </span>
      ),
      enableSorting: false,
    },
    // Delete
    {
      id: "actions",
      header: "",
      cell: ({ row }) => (
        <Button
          size="sm"
          variant="ghost"
          className="h-7 w-7 p-0 text-zinc-600 hover:text-destructive cursor-pointer"
          onClick={(e) => {
            e.preventDefault();
            e.stopPropagation();
            onDelete(row.original.id);
          }}
        >
          <Trash2 className="h-3.5 w-3.5" />
        </Button>
      ),
      enableSorting: false,
      size: 40,
    },
  ];
}

function SortableHeader({
  column,
  label,
}: {
  column: { getIsSorted: () => false | "asc" | "desc"; toggleSorting: (desc?: boolean) => void };
  label: string;
}) {
  const sorted = column.getIsSorted();
  return (
    <button
      className="flex items-center gap-1 hover:text-foreground transition-colors cursor-pointer"
      onClick={() => column.toggleSorting(sorted === "asc")}
    >
      {label}
      {sorted === "asc" ? (
        <ArrowUp className="h-3 w-3" />
      ) : sorted === "desc" ? (
        <ArrowDown className="h-3 w-3" />
      ) : (
        <ArrowUpDown className="h-3 w-3 opacity-40" />
      )}
    </button>
  );
}

// --- Page size options ---
const PAGE_SIZES = [10, 25, 50, 100];

// --- Main component ---

export function Runs() {
  const navigate = useNavigate();
  const [runs, setRuns] = useState<RunSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Table state
  const [sorting, setSorting] = useState<SortingState>([{ id: "started_at", desc: true }]);
  const [columnFilters, setColumnFilters] = useState<ColumnFiltersState>([]);
  const [rowSelection, setRowSelection] = useState<RowSelectionState>({});

  // Comparison
  const [comparing, setComparing] = useState(false);
  const [compareRows, setCompareRows] = useState<ComparisonRow[] | null>(null);
  const [compareError, setCompareError] = useState<string | null>(null);
  const [grafana, setGrafana] = useState<GrafanaSettings | null>(null);

  // Derive unique filter values
  const filterValues = useMemo(() => {
    const statuses = new Set<string>();
    const dbKinds = new Set<string>();
    const providers = new Set<string>();
    for (const r of runs) {
      statuses.add(deriveStatus(r));
      if (r.db_kind) dbKinds.add(r.db_kind);
      if (r.provider) providers.add(r.provider);
    }
    return {
      statuses: Array.from(statuses).sort(),
      dbKinds: Array.from(dbKinds).sort(),
      providers: Array.from(providers).sort(),
    };
  }, [runs]);

  // Active filter helpers
  const activeStatus = columnFilters.find((f) => f.id === "status")?.value as string | undefined;
  const activeDbKind = columnFilters.find((f) => f.id === "db_kind")?.value as string | undefined;
  const activeProvider = columnFilters.find((f) => f.id === "provider")?.value as string | undefined;

  function setFilter(id: string, value: string | undefined) {
    setColumnFilters((prev) => {
      const without = prev.filter((f) => f.id !== id);
      if (!value || value === "all") return without;
      return [...without, { id, value }];
    });
  }

  async function fetchRuns() {
    setLoading(true);
    setError(null);
    try {
      const result = await listRuns();
      setRuns(result ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load runs");
    } finally {
      setLoading(false);
    }
  }

  async function handleDelete(runID: string) {
    if (!confirm(`Delete run "${runID}"? This will also remove its Docker resources.`)) return;
    try {
      await deleteRun(runID);
      await fetchRuns();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete run");
    }
  }

  useEffect(() => {
    fetchRuns();
    getGrafanaSettings().then(setGrafana).catch(() => setGrafana(null));
  }, []);

  const columns = useMemo(() => makeColumns(handleDelete), []);

  const table = useReactTable({
    data: runs,
    columns,
    state: { sorting, columnFilters, rowSelection },
    onSortingChange: setSorting,
    onColumnFiltersChange: setColumnFilters,
    onRowSelectionChange: setRowSelection,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    getRowId: (row) => row.id,
    initialState: {
      pagination: { pageSize: 25 },
    },
  });

  // Selected runs for comparison
  const selectedRunIds = Object.keys(rowSelection).filter((k) => rowSelection[k]);
  const canCompare = selectedRunIds.length === 2;

  async function handleCompare() {
    if (!canCompare) return;
    const [a, b] = selectedRunIds;
    const runA = runs.find((r) => r.id === a);
    const runB = runs.find((r) => r.id === b);
    // Use timestamps from both runs to derive time range
    const starts = [runA?.started_at, runB?.started_at].filter(Boolean).map((t) => new Date(t!).getTime());
    const ends = [runA?.finished_at, runB?.finished_at].filter(Boolean).map((t) => new Date(t!).getTime());
    const start = starts.length > 0 ? new Date(Math.min(...starts)).toISOString() : new Date(Date.now() - 7200000).toISOString();
    const end = ends.length > 0 ? new Date(Math.max(...ends)).toISOString() : new Date().toISOString();

    setComparing(true);
    setCompareError(null);
    setCompareRows(null);
    try {
      const rows = await compareRuns(a, b, start, end);
      setCompareRows(rows);
    } catch (err) {
      setCompareError(err instanceof Error ? err.message : "Comparison failed");
    } finally {
      setComparing(false);
    }
  }

  const hasActiveFilters = activeStatus || activeDbKind || activeProvider;

  return (
    <div className="p-5 space-y-4">
      {/* Header bar */}
      <div className="flex items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <h1 className="text-base font-semibold font-mono tracking-tight">Runs</h1>
          <span className="text-[11px] text-zinc-600 font-mono tabular-nums">
            {table.getFilteredRowModel().rows.length} of {runs.length}
          </span>
        </div>

        <div className="flex items-center gap-2">
          {/* Compare button */}
          {selectedRunIds.length > 0 && (
            <div className="flex items-center gap-2 mr-2">
              <span className="text-[11px] text-zinc-500 font-mono">
                {selectedRunIds.length} selected
              </span>
              {canCompare && (
                <Button
                  size="sm"
                  variant="outline"
                  onClick={handleCompare}
                  disabled={comparing}
                  className="border-primary/40 text-primary hover:bg-primary/10"
                >
                  <GitCompare className="h-3.5 w-3.5" />
                  {comparing ? "Comparing..." : "Compare"}
                </Button>
              )}
              <Button
                size="sm"
                variant="ghost"
                onClick={() => setRowSelection({})}
                className="h-7 w-7 p-0 text-zinc-500"
              >
                <X className="h-3.5 w-3.5" />
              </Button>
            </div>
          )}

          <Button
            size="sm"
            variant="outline"
            onClick={() => navigate("/runs/new")}
          >
            <Play className="h-3.5 w-3.5" />
            New Run
          </Button>

          <Button size="sm" variant="outline" onClick={fetchRuns} disabled={loading}>
            <RefreshCw className={`h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
          </Button>
        </div>
      </div>

      {error && (
        <div className="flex items-center gap-2 text-xs p-2.5 border border-destructive/30 text-destructive font-mono">
          <AlertCircle className="h-3.5 w-3.5 shrink-0" />
          {error}
        </div>
      )}

      {/* Filters */}
      <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
        {/* Status filter */}
        {filterValues.statuses.length > 1 && (
          <div className="flex items-center gap-1.5">
            <span className="text-[10px] text-zinc-600 font-mono uppercase tracking-wider">Status</span>
            <FilterChip label="all" active={!activeStatus} onClick={() => setFilter("status", undefined)} />
            {filterValues.statuses.map((s) => (
              <FilterChip
                key={s}
                label={s}
                active={activeStatus === s}
                onClick={() => setFilter("status", activeStatus === s ? undefined : s)}
              />
            ))}
          </div>
        )}

        {/* DB Kind filter */}
        {filterValues.dbKinds.length > 1 && (
          <div className="flex items-center gap-1.5">
            <span className="text-[10px] text-zinc-600 font-mono uppercase tracking-wider">DB</span>
            <FilterChip label="all" active={!activeDbKind} onClick={() => setFilter("db_kind", undefined)} />
            {filterValues.dbKinds.map((k) => (
              <FilterChip
                key={k}
                label={k}
                active={activeDbKind === k}
                onClick={() => setFilter("db_kind", activeDbKind === k ? undefined : k)}
              />
            ))}
          </div>
        )}

        {/* Provider filter */}
        {filterValues.providers.length > 1 && (
          <div className="flex items-center gap-1.5">
            <span className="text-[10px] text-zinc-600 font-mono uppercase tracking-wider">Provider</span>
            <FilterChip label="all" active={!activeProvider} onClick={() => setFilter("provider", undefined)} />
            {filterValues.providers.map((p) => (
              <FilterChip
                key={p}
                label={p}
                active={activeProvider === p}
                onClick={() => setFilter("provider", activeProvider === p ? undefined : p)}
              />
            ))}
          </div>
        )}

        {hasActiveFilters && (
          <button
            onClick={() => setColumnFilters([])}
            className="text-[10px] text-zinc-500 hover:text-zinc-300 font-mono underline underline-offset-2 cursor-pointer"
          >
            clear filters
          </button>
        )}
      </div>

      {/* Table */}
      <div className="border border-zinc-800/80 bg-[#080808]">
        <Table>
          <TableHeader>
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id} className="border-zinc-800/80 hover:bg-transparent">
                {headerGroup.headers.map((header) => (
                  <TableHead
                    key={header.id}
                    className="text-[11px] font-mono uppercase tracking-wider text-zinc-500 h-9 bg-zinc-900/50"
                    style={header.column.getSize() !== 150 ? { width: header.column.getSize() } : undefined}
                  >
                    {header.isPlaceholder
                      ? null
                      : flexRender(header.column.columnDef.header, header.getContext())}
                  </TableHead>
                ))}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {table.getRowModel().rows.length ? (
              table.getRowModel().rows.map((row) => (
                <TableRow
                  key={row.id}
                  data-state={row.getIsSelected() ? "selected" : undefined}
                  className="border-zinc-800/50 hover:bg-zinc-900/50 data-[state=selected]:bg-primary/[0.04]"
                >
                  {row.getVisibleCells().map((cell) => (
                    <TableCell key={cell.id} className="py-2.5">
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : (
              <TableRow>
                <TableCell colSpan={columns.length} className="h-32 text-center">
                  <span className="text-xs text-zinc-600 font-mono">
                    {loading
                      ? "Loading runs..."
                      : runs.length === 0
                        ? "No runs yet"
                        : "No runs matching filters"}
                  </span>
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>

      {/* Pagination */}
      {runs.length > 0 && (
        <div className="flex items-center justify-between">
          {/* Page size selector */}
          <div className="flex items-center gap-2">
            <span className="text-[11px] text-zinc-600 font-mono">Rows</span>
            <div className="flex gap-0.5">
              {PAGE_SIZES.map((size) => (
                <button
                  key={size}
                  onClick={() => table.setPageSize(size)}
                  className={`px-2 py-0.5 text-[11px] font-mono border transition-colors cursor-pointer ${
                    table.getState().pagination.pageSize === size
                      ? "border-zinc-600 text-zinc-300 bg-zinc-800"
                      : "border-transparent text-zinc-600 hover:text-zinc-400"
                  }`}
                >
                  {size}
                </button>
              ))}
            </div>
          </div>

          {/* Page navigation */}
          <div className="flex items-center gap-1">
            <span className="text-[11px] text-zinc-600 font-mono tabular-nums mr-2">
              {table.getState().pagination.pageIndex * table.getState().pagination.pageSize + 1}
              {"\u2013"}
              {Math.min(
                (table.getState().pagination.pageIndex + 1) * table.getState().pagination.pageSize,
                table.getFilteredRowModel().rows.length
              )}
              {" of "}
              {table.getFilteredRowModel().rows.length}
            </span>
            <Button
              variant="ghost"
              size="sm"
              className="h-7 w-7 p-0"
              onClick={() => table.setPageIndex(0)}
              disabled={!table.getCanPreviousPage()}
            >
              <ChevronsLeft className="h-3.5 w-3.5" />
            </Button>
            <Button
              variant="ghost"
              size="sm"
              className="h-7 w-7 p-0"
              onClick={() => table.previousPage()}
              disabled={!table.getCanPreviousPage()}
            >
              <ChevronLeft className="h-3.5 w-3.5" />
            </Button>
            <span className="text-[11px] text-zinc-500 font-mono tabular-nums px-2">
              {table.getState().pagination.pageIndex + 1}/{table.getPageCount()}
            </span>
            <Button
              variant="ghost"
              size="sm"
              className="h-7 w-7 p-0"
              onClick={() => table.nextPage()}
              disabled={!table.getCanNextPage()}
            >
              <ChevronRight className="h-3.5 w-3.5" />
            </Button>
            <Button
              variant="ghost"
              size="sm"
              className="h-7 w-7 p-0"
              onClick={() => table.setPageIndex(table.getPageCount() - 1)}
              disabled={!table.getCanNextPage()}
            >
              <ChevronsRight className="h-3.5 w-3.5" />
            </Button>
          </div>
        </div>
      )}

      {/* Comparison results panel */}
      {(compareRows || compareError) && (
        <div className="border border-zinc-800/80 bg-[#080808]">
          <div className="flex items-center justify-between px-4 py-2.5 border-b border-zinc-800/50 bg-zinc-900/30">
            <div className="flex items-center gap-2">
              <GitCompare className="h-3.5 w-3.5 text-primary" />
              <span className="text-xs font-mono text-zinc-300">
                {selectedRunIds[0]} vs {selectedRunIds[1]}
              </span>
            </div>
            <div className="flex items-center gap-2">
              {grafana?.embed_enabled && grafana.dashboards?.compare && (
                <a
                  href={`${grafana.url}/d/${grafana.dashboards.compare}?var-run_a=${selectedRunIds[0]}&var-run_b=${selectedRunIds[1]}&kiosk&theme=dark`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-[10px] font-mono text-primary hover:underline"
                >
                  Open in Grafana
                </a>
              )}
              <button
                onClick={() => { setCompareRows(null); setCompareError(null); }}
                className="text-zinc-500 hover:text-zinc-300 cursor-pointer"
              >
                <X className="h-3.5 w-3.5" />
              </button>
            </div>
          </div>
          {compareError ? (
            <div className="p-4 text-xs text-destructive font-mono">{compareError}</div>
          ) : compareRows && compareRows.length > 0 ? (
            <MetricsDiff rows={compareRows} />
          ) : (
            <div className="p-4 text-xs text-zinc-600 font-mono">No comparison data available.</div>
          )}
        </div>
      )}
    </div>
  );
}
