import type { ComparisonRow } from "@/api/types";

interface MetricsDiffProps {
  rows: ComparisonRow[];
}

export function MetricsDiff({ rows }: MetricsDiffProps) {
  if (rows.length === 0) {
    return (
      <div className="text-sm text-muted-foreground p-4">
        No comparison data available.
      </div>
    );
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b text-left text-xs text-muted-foreground">
            <th className="py-2 px-3 font-medium">Metric</th>
            <th className="py-2 px-3 font-medium text-right">Run A</th>
            <th className="py-2 px-3 font-medium text-right">Run B</th>
            <th className="py-2 px-3 font-medium text-right">Diff %</th>
            <th className="py-2 px-3 font-medium">Verdict</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row, i) => (
            <tr key={i} className="border-b border-border/50 hover:bg-muted/30">
              <td className="py-2 px-3 font-mono text-xs">{row.metric}</td>
              <td className="py-2 px-3 text-right font-mono text-xs">
                {row.a.toFixed(2)}
              </td>
              <td className="py-2 px-3 text-right font-mono text-xs">
                {row.b.toFixed(2)}
              </td>
              <td className="py-2 px-3 text-right font-mono text-xs">
                <span
                  className={
                    row.diff_pct > 0
                      ? "text-success"
                      : row.diff_pct < 0
                        ? "text-destructive"
                        : "text-muted-foreground"
                  }
                >
                  {row.diff_pct > 0 ? "+" : ""}
                  {row.diff_pct.toFixed(1)}%
                </span>
              </td>
              <td className="py-2 px-3">
                <span
                  className={`text-xs font-medium ${
                    row.verdict === "better"
                      ? "text-success"
                      : row.verdict === "worse"
                        ? "text-destructive"
                        : "text-muted-foreground"
                  }`}
                >
                  {row.verdict}
                </span>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
