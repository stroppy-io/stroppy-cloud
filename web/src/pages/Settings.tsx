import { useEffect, useState } from "react";
import {
  getSettings,
  updateSettings,
  getPackages,
  updatePackages,
  uploadDeb,
} from "@/api/client";
import type { ServerSettings, PackageDefaults } from "@/api/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { Badge } from "@/components/ui/badge";
import { Save, Upload, AlertCircle, Check } from "lucide-react";

export function SettingsPage() {
  const [settings, setSettings] = useState<ServerSettings | null>(null);
  const [packages, setPackages] = useState<PackageDefaults | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState<{
    type: "success" | "error";
    text: string;
  } | null>(null);
  const [uploadResult, setUploadResult] = useState<string | null>(null);
  const [uploading, setUploading] = useState(false);

  useEffect(() => {
    async function load() {
      try {
        const [s, p] = await Promise.all([getSettings(), getPackages()]);
        setSettings(s);
        setPackages(p);
      } catch (err) {
        setMessage({
          type: "error",
          text: err instanceof Error ? err.message : "Failed to load settings",
        });
      } finally {
        setLoading(false);
      }
    }
    load();
  }, []);

  async function handleSaveSettings() {
    if (!settings) return;
    setSaving(true);
    setMessage(null);
    try {
      await updateSettings(settings);
      setMessage({ type: "success", text: "Settings saved" });
    } catch (err) {
      setMessage({
        type: "error",
        text: err instanceof Error ? err.message : "Failed to save",
      });
    }
    setSaving(false);
  }

  async function handleSavePackages() {
    if (!packages) return;
    setSaving(true);
    setMessage(null);
    try {
      await updatePackages(packages);
      setMessage({ type: "success", text: "Packages saved" });
    } catch (err) {
      setMessage({
        type: "error",
        text: err instanceof Error ? err.message : "Failed to save",
      });
    }
    setSaving(false);
  }

  async function handleUpload(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    setUploading(true);
    setUploadResult(null);
    try {
      const result = await uploadDeb(file);
      setUploadResult(
        `Uploaded ${result.filename} (${result.size} bytes) -> ${result.url}`
      );
    } catch (err) {
      setUploadResult(
        `Error: ${err instanceof Error ? err.message : "Upload failed"}`
      );
    }
    setUploading(false);
  }

  function updateField<
    S extends keyof ServerSettings,
    K extends keyof ServerSettings[S],
  >(section: S, key: K, value: ServerSettings[S][K]) {
    if (!settings) return;
    setSettings({
      ...settings,
      [section]: { ...settings[section], [key]: value },
    });
  }

  if (loading) {
    return (
      <div className="p-6 text-sm text-muted-foreground">
        Loading settings...
      </div>
    );
  }

  return (
    <div className="p-6 space-y-6">
      <div>
        <h1 className="text-lg font-semibold">Settings</h1>
        <p className="text-sm text-muted-foreground">
          Server configuration and package management
        </p>
      </div>

      {message && (
        <div
          className={`flex items-center gap-2 text-sm p-3 border ${
            message.type === "success"
              ? "border-success/30 text-success"
              : "border-destructive/30 text-destructive"
          }`}
        >
          {message.type === "success" ? (
            <Check className="h-4 w-4" />
          ) : (
            <AlertCircle className="h-4 w-4" />
          )}
          {message.text}
        </div>
      )}

      <Tabs defaultValue="cloud">
        <TabsList>
          <TabsTrigger value="cloud">Cloud</TabsTrigger>
          <TabsTrigger value="monitoring">Monitoring</TabsTrigger>
          <TabsTrigger value="stroppy">Stroppy / OTLP</TabsTrigger>
          <TabsTrigger value="packages">Packages</TabsTrigger>
          <TabsTrigger value="upload">Upload .deb</TabsTrigger>
        </TabsList>

        {/* Cloud settings */}
        <TabsContent value="cloud">
          {settings && (
            <Card>
              <CardHeader>
                <CardTitle>Yandex Cloud</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="grid grid-cols-2 gap-4">
                  {(
                    [
                      ["folder_id", "Folder ID"],
                      ["zone", "Zone"],
                      ["subnet_id", "Subnet ID"],
                      ["service_account_id", "Service Account ID"],
                      ["image_id", "Image ID"],
                    ] as const
                  ).map(([key, label]) => (
                    <div key={key} className="space-y-2">
                      <Label>{label}</Label>
                      <Input
                        value={(settings.cloud.yandex as any)[key] || ""}
                        onChange={(e) =>
                          setSettings({
                            ...settings,
                            cloud: {
                              ...settings.cloud,
                              yandex: {
                                ...settings.cloud.yandex,
                                [key]: e.target.value,
                              },
                            },
                          })
                        }
                        className="font-mono text-xs"
                      />
                    </div>
                  ))}
                </div>
                <div className="space-y-2">
                  <Label>SSH Public Key</Label>
                  <textarea
                    className="w-full h-20 bg-transparent border border-input p-3 font-mono text-xs resize-y focus:outline-none focus:ring-1 focus:ring-ring"
                    value={settings.cloud.yandex.ssh_public_key || ""}
                    onChange={(e) =>
                      setSettings({
                        ...settings,
                        cloud: {
                          ...settings.cloud,
                          yandex: {
                            ...settings.cloud.yandex,
                            ssh_public_key: e.target.value,
                          },
                        },
                      })
                    }
                  />
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <Label>Server Address</Label>
                    <Input
                      value={settings.cloud.server_addr || ""}
                      onChange={(e) =>
                        updateField("cloud", "server_addr", e.target.value)
                      }
                      className="font-mono text-xs"
                      placeholder="http://my-server:8080"
                    />
                  </div>
                  <div className="space-y-2">
                    <Label>Binary URL Override</Label>
                    <Input
                      value={settings.cloud.binary_url || ""}
                      onChange={(e) =>
                        updateField("cloud", "binary_url", e.target.value)
                      }
                      className="font-mono text-xs"
                    />
                  </div>
                </div>
                <Button onClick={handleSaveSettings} disabled={saving}>
                  <Save className="h-3.5 w-3.5" />
                  {saving ? "Saving..." : "Save Settings"}
                </Button>
              </CardContent>
            </Card>
          )}
        </TabsContent>

        {/* Monitoring */}
        <TabsContent value="monitoring">
          {settings && (
            <Card>
              <CardHeader>
                <CardTitle>Monitoring Stack</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="grid grid-cols-2 gap-4">
                  {(
                    [
                      ["node_exporter_version", "Node Exporter Version"],
                      ["postgres_exporter_version", "Postgres Exporter Version"],
                      ["otel_col_version", "OTel Collector Version"],
                      ["vmagent_version", "VMAgent Version"],
                      ["victoria_metrics_url", "VictoriaMetrics URL"],
                      ["victoria_metrics_user", "VictoriaMetrics User"],
                    ] as const
                  ).map(([key, label]) => (
                    <div key={key} className="space-y-2">
                      <Label>{label}</Label>
                      <Input
                        value={
                          (settings.monitoring as any)[key] || ""
                        }
                        onChange={(e) =>
                          setSettings({
                            ...settings,
                            monitoring: {
                              ...settings.monitoring,
                              [key]: e.target.value,
                            },
                          })
                        }
                        className="font-mono text-xs"
                      />
                    </div>
                  ))}
                  <div className="space-y-2">
                    <Label>VictoriaMetrics Password</Label>
                    <Input
                      type="password"
                      value={settings.monitoring.victoria_metrics_password || ""}
                      onChange={(e) =>
                        setSettings({
                          ...settings,
                          monitoring: {
                            ...settings.monitoring,
                            victoria_metrics_password: e.target.value,
                          },
                        })
                      }
                      className="font-mono text-xs"
                    />
                  </div>
                </div>
                <Button onClick={handleSaveSettings} disabled={saving}>
                  <Save className="h-3.5 w-3.5" />
                  {saving ? "Saving..." : "Save Settings"}
                </Button>
              </CardContent>
            </Card>
          )}
        </TabsContent>

        {/* Stroppy / OTLP */}
        <TabsContent value="stroppy">
          {settings && (
            <Card>
              <CardHeader>
                <CardTitle>Stroppy Defaults / OTLP</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="grid grid-cols-2 gap-4">
                  {(
                    [
                      ["version", "Stroppy Version"],
                      ["otlp_exporter_type", "OTLP Exporter Type (http/grpc)"],
                      ["otlp_endpoint", "OTLP Endpoint"],
                      ["otlp_url_path", "OTLP URL Path"],
                      ["otlp_headers", "OTLP Headers"],
                      ["otlp_metric_prefix", "OTLP Metric Prefix"],
                      ["otlp_service_name", "OTLP Service Name"],
                    ] as const
                  ).map(([key, label]) => (
                    <div key={key} className="space-y-2">
                      <Label>{label}</Label>
                      <Input
                        value={
                          (settings.stroppy_defaults as any)[
                            key
                          ] || ""
                        }
                        onChange={(e) =>
                          setSettings({
                            ...settings,
                            stroppy_defaults: {
                              ...settings.stroppy_defaults,
                              [key]: e.target.value,
                            },
                          })
                        }
                        className="font-mono text-xs"
                      />
                    </div>
                  ))}
                  <div className="space-y-2">
                    <Label>OTLP Insecure</Label>
                    <div className="flex items-center gap-2 h-9">
                      <input
                        type="checkbox"
                        checked={settings.stroppy_defaults.otlp_insecure}
                        onChange={(e) =>
                          setSettings({
                            ...settings,
                            stroppy_defaults: {
                              ...settings.stroppy_defaults,
                              otlp_insecure: e.target.checked,
                            },
                          })
                        }
                        className="accent-primary"
                      />
                      <span className="text-sm text-muted-foreground">
                        Allow insecure connections
                      </span>
                    </div>
                  </div>
                </div>
                <Button onClick={handleSaveSettings} disabled={saving}>
                  <Save className="h-3.5 w-3.5" />
                  {saving ? "Saving..." : "Save Settings"}
                </Button>
              </CardContent>
            </Card>
          )}
        </TabsContent>

        {/* Packages */}
        <TabsContent value="packages">
          {packages && (
            <Card>
              <CardHeader>
                <CardTitle>Package Defaults</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <p className="text-xs text-muted-foreground">
                  Edit the raw JSON for package defaults. Changes apply to all
                  new runs.
                </p>
                <textarea
                  className="w-full h-96 bg-[#050505] border border-input p-3 font-mono text-xs text-foreground resize-y focus:outline-none focus:ring-1 focus:ring-ring"
                  value={JSON.stringify(packages, null, 2)}
                  onChange={(e) => {
                    try {
                      setPackages(JSON.parse(e.target.value));
                    } catch {
                      // let user keep typing
                    }
                  }}
                />
                <Button onClick={handleSavePackages} disabled={saving}>
                  <Save className="h-3.5 w-3.5" />
                  {saving ? "Saving..." : "Save Packages"}
                </Button>
              </CardContent>
            </Card>
          )}
        </TabsContent>

        {/* Upload */}
        <TabsContent value="upload">
          <Card>
            <CardHeader>
              <CardTitle>Upload .deb Package</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <p className="text-xs text-muted-foreground">
                Upload a .deb file. It will be served at /packages/ for agents
                to download.
              </p>
              <div className="flex items-center gap-3">
                <label className="cursor-pointer">
                  <input
                    type="file"
                    accept=".deb"
                    onChange={handleUpload}
                    className="hidden"
                  />
                  <Button asChild disabled={uploading} variant="outline">
                    <span>
                      <Upload className="h-3.5 w-3.5" />
                      {uploading ? "Uploading..." : "Choose .deb file"}
                    </span>
                  </Button>
                </label>
              </div>
              {uploadResult && (
                <div className="p-3 border text-xs font-mono">
                  {uploadResult}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}
