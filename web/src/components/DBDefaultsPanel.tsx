import { useEffect, useState } from "react";
import { getDBDefaults } from "@/api/client";
import type { DatabaseKind } from "@/api/types";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Database, Server, Cpu } from "lucide-react";

interface DBDefaultsData {
  [version: string]: Record<string, string>;
}

const kindIcons: Record<DatabaseKind, typeof Database> = {
  postgres: Database,
  mysql: Server,
  picodata: Cpu,
};

const kindLabels: Record<DatabaseKind, string> = {
  postgres: "PostgreSQL",
  mysql: "MySQL",
  picodata: "Picodata",
};

const highlightKeys: Record<DatabaseKind, string[]> = {
  postgres: ["shared_buffers", "max_connections", "max_wal_size", "listen_addresses"],
  mysql: ["innodb_buffer_pool_size", "max_connections", "innodb_flush_method", "bind_address"],
  picodata: ["replication_factor", "shards", "memtx_memory", "listen"],
};

interface DBDefaultsPanelProps {
  kind: DatabaseKind;
}

export function DBDefaultsPanel({ kind }: DBDefaultsPanelProps) {
  const [data, setData] = useState<DBDefaultsData | null>(null);
  const [loading, setLoading] = useState(true);
  const [selectedVersion, setSelectedVersion] = useState<string | null>(null);

  useEffect(() => {
    setLoading(true);
    getDBDefaults(kind)
      .then((resp) => {
        setData(resp as unknown as DBDefaultsData);
        const versions = Object.keys(resp);
        if (versions.length > 0) setSelectedVersion(versions[0]);
      })
      .catch(() => setData(null))
      .finally(() => setLoading(false));
  }, [kind]);

  if (loading) {
    return (
      <div className="text-xs text-zinc-600 font-mono py-2">
        Loading defaults...
      </div>
    );
  }

  if (!data || Object.keys(data).length === 0) {
    return (
      <div className="text-xs text-zinc-600 font-mono py-2">
        No configuration defaults available.
      </div>
    );
  }

  const versions = Object.keys(data);
  const Icon = kindIcons[kind];
  const highlighted = highlightKeys[kind] || [];

  return (
    <Card className="border-zinc-800/40 bg-[#070707]">
      <CardHeader className="pb-2 pt-3 px-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Icon className="h-3.5 w-3.5 text-zinc-500" />
            <CardTitle className="text-xs font-normal text-zinc-400">
              {kindLabels[kind]} Configuration Defaults
            </CardTitle>
          </div>
          <div className="flex gap-1">
            {versions.map((v) => (
              <button
                key={v}
                onClick={() => setSelectedVersion(v)}
                className={`px-2 py-0.5 text-[10px] font-mono border transition-colors ${
                  selectedVersion === v
                    ? "border-primary/50 text-primary bg-primary/5"
                    : "border-zinc-800 text-zinc-600 hover:text-zinc-400 hover:border-zinc-700"
                }`}
              >
                v{v}
              </button>
            ))}
          </div>
        </div>
      </CardHeader>
      <CardContent className="px-4 pb-3">
        {selectedVersion && data[selectedVersion] && (
          <div className="space-y-0.5">
            {Object.entries(data[selectedVersion])
              .sort(([a], [b]) => {
                const aH = highlighted.includes(a) ? 0 : 1;
                const bH = highlighted.includes(b) ? 0 : 1;
                return aH - bH || a.localeCompare(b);
              })
              .map(([key, value]) => {
                const isHighlighted = highlighted.includes(key);
                return (
                  <div
                    key={key}
                    className={`flex items-center justify-between py-1 border-b border-zinc-900/30 last:border-0 ${
                      isHighlighted ? "" : "opacity-60"
                    }`}
                  >
                    <div className="flex items-center gap-2">
                      {isHighlighted && (
                        <div className="w-1 h-1 bg-primary" />
                      )}
                      <span
                        className={`text-xs font-mono ${
                          isHighlighted ? "text-zinc-300" : "text-zinc-500"
                        }`}
                      >
                        {key}
                      </span>
                    </div>
                    <span className="text-xs font-mono text-zinc-400">
                      {String(value)}
                    </span>
                  </div>
                );
              })}
            <div className="pt-2 flex items-center gap-1.5">
              <Badge variant="outline" className="text-[9px] h-4">
                {Object.keys(data[selectedVersion]).length} parameters
              </Badge>
              {highlighted.length > 0 && (
                <Badge variant="outline" className="text-[9px] h-4 border-primary/30 text-primary/70">
                  {highlighted.length} key settings
                </Badge>
              )}
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
