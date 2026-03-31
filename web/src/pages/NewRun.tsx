import { useState, useEffect, useMemo } from "react";
import { useSearchParams, useNavigate } from "react-router-dom";
import { startRun, validateRun, dryRun, getPresets } from "@/api/client";
import type {
  RunConfig,
  DatabaseKind,
  Provider,
  PresetsResponse,
} from "@/api/types";
import { generateRunID } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/components/ui/select";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { Badge } from "@/components/ui/badge";
import { TopologyDiagram } from "@/components/TopologyDiagram";
import { Play, Eye, Check, AlertCircle, ChevronDown, ChevronRight } from "lucide-react";

const DB_KINDS: DatabaseKind[] = ["postgres", "mysql", "picodata"];
const PROVIDERS: Provider[] = ["yandex", "docker"];
const WORKLOADS = ["simple", "tpcb", "tpcc"];

const DB_VERSIONS: Record<DatabaseKind, string[]> = {
  postgres: ["16", "17"],
  mysql: ["8.0", "8.4"],
  picodata: ["25.3"],
};

const PRESET_NAMES: Record<DatabaseKind, string[]> = {
  postgres: ["single", "ha", "scale"],
  mysql: ["single", "replica", "group"],
  picodata: ["single", "cluster", "scale"],
};

export function NewRun() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();

  const [presets, setPresets] = useState<PresetsResponse | null>(null);
  const [kind, setKind] = useState<DatabaseKind>(
    (searchParams.get("kind") as DatabaseKind) || "postgres"
  );
  const [preset, setPreset] = useState(
    searchParams.get("preset") || "single"
  );
  const [provider, setProvider] = useState<Provider>("docker");
  const [version, setVersion] = useState(DB_VERSIONS[kind][0]);
  const [workload, setWorkload] = useState("simple");
  const [duration, setDuration] = useState("5m");
  const [workers, setWorkers] = useState(4);
  const [cidr, setCidr] = useState("10.0.0.0/24");
  const [showPackages, setShowPackages] = useState(false);
  const [customPackagesJSON, setCustomPackagesJSON] = useState("");

  const [submitting, setSubmitting] = useState(false);
  const [validating, setValidating] = useState(false);
  const [validationResult, setValidationResult] = useState<{
    ok: boolean;
    message: string;
  } | null>(null);
  const [dryRunResult, setDryRunResult] = useState<unknown | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getPresets().then(setPresets).catch(() => {});
  }, []);

  // Reset version/preset when kind changes
  useEffect(() => {
    setVersion(DB_VERSIONS[kind][0]);
    setPreset(PRESET_NAMES[kind][0]);
  }, [kind]);

  const config = useMemo((): RunConfig => {
    const id = generateRunID();

    const buildTopology = () => {
      if (presets) {
        const p =
          kind === "postgres"
            ? presets.postgres[preset]
            : kind === "mysql"
              ? presets.mysql[preset]
              : presets.picodata[preset];
        if (p) {
          if (kind === "postgres") return { postgres: p };
          if (kind === "mysql") return { mysql: p };
          if (kind === "picodata") return { picodata: p };
        }
      }
      return {};
    };

    const topo = buildTopology();
    const cfg: RunConfig = {
      id,
      provider,
      network: { cidr },
      machines: [],
      database: {
        kind,
        version,
        ...topo,
      } as RunConfig["database"],
      monitor: {},
      stroppy: {
        version: "3.1.0",
        workload,
        duration,
        workers,
      },
    };

    if (customPackagesJSON.trim()) {
      try {
        cfg.packages = JSON.parse(customPackagesJSON);
      } catch {
        // ignore parse errors for preview
      }
    }

    return cfg;
  }, [kind, preset, provider, version, workload, duration, workers, cidr, presets, customPackagesJSON]);

  async function handleValidate() {
    setValidating(true);
    setValidationResult(null);
    try {
      await validateRun(config);
      setValidationResult({ ok: true, message: "Configuration is valid" });
    } catch (err) {
      setValidationResult({
        ok: false,
        message: err instanceof Error ? err.message : "Validation failed",
      });
    }
    setValidating(false);
  }

  async function handleDryRun() {
    setDryRunResult(null);
    try {
      const result = await dryRun(config);
      setDryRunResult(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Dry run failed");
    }
  }

  async function handleSubmit() {
    setSubmitting(true);
    setError(null);
    try {
      const result = await startRun(config);
      navigate(`/runs/${result.run_id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to start run");
      setSubmitting(false);
    }
  }

  return (
    <div className="p-6">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-lg font-semibold">New Run</h1>
          <p className="text-sm text-muted-foreground">
            Configure and launch a database test run
          </p>
        </div>
      </div>

      <div className="grid grid-cols-[1fr_400px] gap-6">
        {/* Left: form */}
        <div className="space-y-6">
          {/* DB Kind */}
          <Card>
            <CardHeader>
              <CardTitle>Database</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid grid-cols-3 gap-3">
                {DB_KINDS.map((k) => (
                  <button
                    key={k}
                    onClick={() => setKind(k)}
                    className={`border p-3 text-sm font-medium text-left transition-colors ${
                      kind === k
                        ? "border-primary text-foreground bg-primary/5"
                        : "border-border text-muted-foreground hover:border-border hover:bg-muted/30"
                    }`}
                  >
                    {k.charAt(0).toUpperCase() + k.slice(1)}
                  </button>
                ))}
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-2">
                  <Label>Version</Label>
                  <Select value={version} onValueChange={setVersion}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {DB_VERSIONS[kind].map((v) => (
                        <SelectItem key={v} value={v}>
                          {v}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-2">
                  <Label>Provider</Label>
                  <Select
                    value={provider}
                    onValueChange={(v) => setProvider(v as Provider)}
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {PROVIDERS.map((p) => (
                        <SelectItem key={p} value={p}>
                          {p}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </div>
            </CardContent>
          </Card>

          {/* Topology preset */}
          <Card>
            <CardHeader>
              <CardTitle>Topology</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="grid grid-cols-3 gap-3">
                {PRESET_NAMES[kind].map((p) => (
                  <button
                    key={p}
                    onClick={() => setPreset(p)}
                    className={`border p-4 text-left transition-colors ${
                      preset === p
                        ? "border-primary bg-primary/5"
                        : "border-border hover:bg-muted/30"
                    }`}
                  >
                    <div className="text-sm font-medium mb-2">{p}</div>
                    <TopologyDiagram kind={kind} preset={p} />
                  </button>
                ))}
              </div>
            </CardContent>
          </Card>

          {/* Stroppy config */}
          <Card>
            <CardHeader>
              <CardTitle>Workload</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid grid-cols-3 gap-3">
                {WORKLOADS.map((w) => (
                  <button
                    key={w}
                    onClick={() => setWorkload(w)}
                    className={`border p-3 text-sm font-medium transition-colors ${
                      workload === w
                        ? "border-primary text-foreground bg-primary/5"
                        : "border-border text-muted-foreground hover:bg-muted/30"
                    }`}
                  >
                    {w.toUpperCase()}
                  </button>
                ))}
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-2">
                  <Label>Duration</Label>
                  <Input
                    value={duration}
                    onChange={(e) => setDuration(e.target.value)}
                    placeholder="5m"
                  />
                </div>
                <div className="space-y-2">
                  <Label>Workers: {workers}</Label>
                  <input
                    type="range"
                    min={1}
                    max={64}
                    value={workers}
                    onChange={(e) => setWorkers(Number(e.target.value))}
                    className="w-full h-1 bg-border appearance-none cursor-pointer accent-primary"
                  />
                  <div className="flex justify-between text-[10px] text-muted-foreground">
                    <span>1</span>
                    <span>64</span>
                  </div>
                </div>
              </div>

              <div className="space-y-2">
                <Label>Network CIDR</Label>
                <Input
                  value={cidr}
                  onChange={(e) => setCidr(e.target.value)}
                  placeholder="10.0.0.0/24"
                />
              </div>
            </CardContent>
          </Card>

          {/* Custom packages (collapsible) */}
          <Card>
            <CardHeader>
              <button
                className="flex items-center gap-2 w-full text-left"
                onClick={() => setShowPackages(!showPackages)}
              >
                {showPackages ? (
                  <ChevronDown className="h-4 w-4" />
                ) : (
                  <ChevronRight className="h-4 w-4" />
                )}
                <CardTitle>Custom Packages</CardTitle>
                <Badge variant="secondary" className="ml-auto text-[10px]">
                  optional
                </Badge>
              </button>
            </CardHeader>
            {showPackages && (
              <CardContent>
                <Label className="mb-2 block">
                  PackageSet JSON override
                </Label>
                <textarea
                  className="w-full h-32 bg-[#050505] border border-input p-3 font-mono text-xs text-foreground resize-y focus:outline-none focus:ring-1 focus:ring-ring"
                  value={customPackagesJSON}
                  onChange={(e) => setCustomPackagesJSON(e.target.value)}
                  placeholder='{"apt": ["custom-pkg"], "pre_install_apt": ["apt-get update"]}'
                />
              </CardContent>
            )}
          </Card>

          {/* Actions */}
          <div className="flex items-center gap-3">
            <Button onClick={handleSubmit} disabled={submitting}>
              <Play className="h-3.5 w-3.5" />
              {submitting ? "Starting..." : "Start Run"}
            </Button>
            <Button
              variant="outline"
              onClick={handleValidate}
              disabled={validating}
            >
              <Check className="h-3.5 w-3.5" />
              Validate
            </Button>
            <Button variant="outline" onClick={handleDryRun}>
              <Eye className="h-3.5 w-3.5" />
              Dry Run
            </Button>
          </div>

          {validationResult && (
            <div
              className={`flex items-center gap-2 text-sm p-3 border ${
                validationResult.ok
                  ? "border-success/30 text-success"
                  : "border-destructive/30 text-destructive"
              }`}
            >
              {validationResult.ok ? (
                <Check className="h-4 w-4" />
              ) : (
                <AlertCircle className="h-4 w-4" />
              )}
              {validationResult.message}
            </div>
          )}

          {error && (
            <div className="flex items-center gap-2 text-sm p-3 border border-destructive/30 text-destructive">
              <AlertCircle className="h-4 w-4" />
              {error}
            </div>
          )}
        </div>

        {/* Right: JSON preview */}
        <div className="space-y-4">
          <Tabs defaultValue="config">
            <TabsList>
              <TabsTrigger value="config">Config JSON</TabsTrigger>
              <TabsTrigger value="dryrun">Dry Run</TabsTrigger>
            </TabsList>
            <TabsContent value="config">
              <Card>
                <CardContent className="p-0">
                  <pre className="p-4 text-xs font-mono overflow-auto max-h-[calc(100vh-200px)] bg-[#050505]">
                    {JSON.stringify(config, null, 2)}
                  </pre>
                </CardContent>
              </Card>
            </TabsContent>
            <TabsContent value="dryrun">
              <Card>
                <CardContent className="p-0">
                  <pre className="p-4 text-xs font-mono overflow-auto max-h-[calc(100vh-200px)] bg-[#050505]">
                    {dryRunResult
                      ? JSON.stringify(dryRunResult, null, 2)
                      : "Run a dry run to see the execution plan."}
                  </pre>
                </CardContent>
              </Card>
            </TabsContent>
          </Tabs>
        </div>
      </div>
    </div>
  );
}
