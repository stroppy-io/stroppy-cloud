import { useEffect, useState } from "react";
import {
  getSettings,
  updateSettings,
} from "@/api/client";
import type { ServerSettings } from "@/api/types";
import { useAuth } from "@/hooks/useAuth";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { Badge } from "@/components/ui/badge";
import { Save, AlertCircle, Check } from "lucide-react";

export function SettingsPage() {
  const { user } = useAuth();
  const canEdit = !!user && (user.is_root || user.role === "owner");
  const [settings, setSettings] = useState<ServerSettings | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState<{
    type: "success" | "error";
    text: string;
  } | null>(null);

  useEffect(() => {
    async function load() {
      try {
        const s = await getSettings();
        setSettings(s);
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
        </TabsList>

        {/* Cloud settings */}
        <TabsContent value="cloud">
          {settings && (
            <Card>
              <CardHeader>
                <CardTitle>Cloud / Server</CardTitle>
              </CardHeader>
              <CardContent className="space-y-6">
                {/* Server address — required for cloud runs */}
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <Label>
                      Server Address <Badge variant="destructive" className="text-[10px] ml-1">required</Badge>
                      {!user?.is_root && <Badge variant="secondary" className="text-[10px] ml-1">root only</Badge>}
                    </Label>
                    <Input
                      value={settings.cloud.server_addr || ""}
                      onChange={(e) =>
                        updateField("cloud", "server_addr", e.target.value)
                      }
                      disabled={!user?.is_root}
                      className="font-mono text-xs"
                      placeholder="http://84.201.148.157:8080"
                    />
                    <p className="text-[10px] text-muted-foreground">
                      Public URL agents will use to reach this server
                    </p>
                  </div>
                  <div className="space-y-2">
                    <Label>
                      Binary URL Override
                      {!user?.is_root && <Badge variant="secondary" className="text-[10px] ml-1">root only</Badge>}
                    </Label>
                    <Input
                      value={settings.cloud.binary_url || ""}
                      onChange={(e) =>
                        updateField("cloud", "binary_url", e.target.value)
                      }
                      disabled={!user?.is_root}
                      className="font-mono text-xs"
                      placeholder="defaults to server_addr/agent/binary"
                    />
                  </div>
                </div>

                <hr className="border-border" />

                {/* YC Credentials */}
                <div>
                  <h3 className="text-sm font-medium mb-3">Yandex Cloud Credentials</h3>
                  <div className="grid grid-cols-2 gap-4">
                    <div className="space-y-2">
                      <Label>
                        Token (YC_TOKEN) <Badge variant="destructive" className="text-[10px] ml-1">required</Badge>
                      </Label>
                      <Input
                        type="password"
                        value={settings.cloud.yandex.token || ""}
                        onChange={(e) =>
                          setSettings({
                            ...settings,
                            cloud: {
                              ...settings.cloud,
                              yandex: { ...settings.cloud.yandex, token: e.target.value },
                            },
                          })
                        }
                        className="font-mono text-xs"
                        placeholder="OAuth or IAM token"
                      />
                    </div>
                    <div className="space-y-2">
                      <Label>
                        Cloud ID <Badge variant="destructive" className="text-[10px] ml-1">required</Badge>
                      </Label>
                      <Input
                        value={settings.cloud.yandex.cloud_id || ""}
                        onChange={(e) =>
                          setSettings({
                            ...settings,
                            cloud: {
                              ...settings.cloud,
                              yandex: { ...settings.cloud.yandex, cloud_id: e.target.value },
                            },
                          })
                        }
                        className="font-mono text-xs"
                      />
                    </div>
                  </div>
                </div>

                <hr className="border-border" />

                {/* YC Infrastructure */}
                <div>
                  <h3 className="text-sm font-medium mb-3">Yandex Cloud Infrastructure</h3>
                  <div className="grid grid-cols-2 gap-4">
                    {(
                      [
                        ["folder_id", "Folder ID", true],
                        ["zone", "Zone", false],
                        ["network_id", "Network ID (VPC)", true],
                        ["network_name", "Subnet Name", true],
                        ["subnet_cidr", "Subnet CIDR", true],
                        ["platform_id", "Platform ID", false],
                        ["image_id", "Image ID", true],
                      ] as const
                    ).map(([key, label, required]) => (
                      <div key={key} className="space-y-2">
                        <Label>
                          {label}
                          {required && <Badge variant="destructive" className="text-[10px] ml-1">required</Badge>}
                        </Label>
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
                </div>

                <div className="space-y-2">
                  <Label>Assign Public IP</Label>
                  <div className="flex items-center gap-2 h-9">
                    <input
                      type="checkbox"
                      checked={settings.cloud.yandex.assign_public_ip ?? false}
                      onChange={(e) =>
                        setSettings({
                          ...settings,
                          cloud: {
                            ...settings.cloud,
                            yandex: {
                              ...settings.cloud.yandex,
                              assign_public_ip: e.target.checked,
                            },
                          },
                        })
                      }
                      className="accent-primary"
                    />
                    <span className="text-sm text-muted-foreground">
                      Allocate external IP addresses on VMs
                    </span>
                  </div>
                </div>

                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <Label>SSH User</Label>
                    <Input
                      value={settings.cloud.yandex.ssh_user || ""}
                      onChange={(e) =>
                        setSettings({
                          ...settings,
                          cloud: {
                            ...settings.cloud,
                            yandex: {
                              ...settings.cloud.yandex,
                              ssh_user: e.target.value,
                            },
                          },
                        })
                      }
                      className="font-mono text-xs"
                      placeholder="stroppy"
                    />
                    <p className="text-[10px] text-muted-foreground">
                      Login user created on VMs (default: stroppy)
                    </p>
                  </div>
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

                {canEdit && (
                  <Button onClick={handleSaveSettings} disabled={saving}>
                    <Save className="h-3.5 w-3.5" />
                    {saving ? "Saving..." : "Save Settings"}
                  </Button>
                )}
              </CardContent>
            </Card>
          )}
        </TabsContent>

      </Tabs>
    </div>
  );
}
