import { useEffect, useState } from "react"
import { api } from "@/lib/api"
import { Card, CardContent } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Separator } from "@/components/ui/separator"
import { Save, Loader2, Server, Container, Cloud } from "lucide-react"
import { create } from "@bufbuild/protobuf"
import { SettingsSchema, type Settings } from "@/proto/settings/settings_pb.ts"

const sections = [
  { id: "hatchet", label: "Hatchet", icon: Server, desc: "Task scheduler backend" },
  { id: "docker", label: "Docker", icon: Container, desc: "Local Docker backend" },
  { id: "yandex", label: "Yandex Cloud", icon: Cloud, desc: "Cloud provider settings" },
] as const

type SectionId = (typeof sections)[number]["id"]

export function SettingsPage() {
  const [settings, setSettings] = useState<Settings>(create(SettingsSchema, {}))
  const [saving, setSaving] = useState(false)
  const [loaded, setLoaded] = useState(false)
  const [active, setActive] = useState<SectionId>("hatchet")

  useEffect(() => {
    api.getSettings({}).then(res => {
      if (res.settings) setSettings(res.settings)
      setLoaded(true)
    }).catch(() => setLoaded(true))
  }, [])

  const handleSave = async () => {
    setSaving(true)
    try {
      const res = await api.updateSettings({ settings })
      if (res.settings) setSettings(res.settings)
    } catch (err) {
      alert(err instanceof Error ? err.message : "Save failed")
    }
    setSaving(false)
  }

  if (!loaded) return <div className="flex items-center justify-center h-64"><Loader2 className="w-5 h-5 animate-spin text-muted-foreground" /></div>

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">Settings</h1>
          <p className="text-muted-foreground text-sm mt-0.5">Platform configuration</p>
        </div>
        <Button onClick={handleSave} disabled={saving} size="sm">
          {saving ? <Loader2 className="w-3.5 h-3.5 mr-1.5 animate-spin" /> : <Save className="w-3.5 h-3.5 mr-1.5" />}
          Save Changes
        </Button>
      </div>

      {/* Two-column: sidebar nav + content */}
      <div className="grid grid-cols-[200px_1fr] gap-6">
        {/* Section nav sidebar */}
        <nav className="space-y-1">
          {sections.map(s => (
            <button key={s.id} onClick={() => setActive(s.id)}
              className={`w-full flex items-center gap-2.5 px-3 py-2.5 rounded-lg text-left transition-all duration-150 ${
                active === s.id
                  ? "bg-primary/10 text-primary font-medium"
                  : "text-muted-foreground hover:bg-muted/60 hover:text-foreground"
              }`}
            >
              <s.icon className="w-4 h-4 shrink-0" />
              <div className="min-w-0">
                <div className="text-[13px] truncate">{s.label}</div>
                <div className="text-[10px] text-muted-foreground truncate">{s.desc}</div>
              </div>
            </button>
          ))}
        </nav>

        {/* Content area */}
        <div>
          {active === "hatchet" && (
            <Card>
              <CardContent className="pt-6 pb-5 space-y-6">
                <div>
                  <h3 className="text-sm font-medium mb-1">Hatchet Connection</h3>
                  <p className="text-xs text-muted-foreground">Configure your Hatchet task scheduler backend</p>
                </div>
                <Separator />
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-1.5">
                    <Label className="text-xs">Host</Label>
                    <Input value={settings.hatchetConnection?.host ?? ""} onChange={e =>
                      setSettings({ ...settings, hatchetConnection: { ...settings.hatchetConnection!, host: e.target.value } })
                    } placeholder="localhost" />
                  </div>
                  <div className="space-y-1.5">
                    <Label className="text-xs">Port</Label>
                    <Input type="number" value={settings.hatchetConnection?.port ?? 7077} onChange={e =>
                      setSettings({ ...settings, hatchetConnection: { ...settings.hatchetConnection!, port: parseInt(e.target.value) || 0 } })
                    } />
                  </div>
                </div>
                <div className="space-y-1.5">
                  <Label className="text-xs">Token</Label>
                  <Input type="password" value={settings.hatchetConnection?.token ?? ""} onChange={e =>
                    setSettings({ ...settings, hatchetConnection: { ...settings.hatchetConnection!, token: e.target.value } })
                  } placeholder="••••••••" />
                </div>
              </CardContent>
            </Card>
          )}

          {active === "docker" && (
            <Card>
              <CardContent className="pt-6 pb-5 space-y-6">
                <div>
                  <h3 className="text-sm font-medium mb-1">Docker Settings</h3>
                  <p className="text-xs text-muted-foreground">Local Docker backend configuration</p>
                </div>
                <Separator />
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-1.5">
                    <Label className="text-xs">Network Name</Label>
                    <Input value={settings.docker?.networkName ?? ""} onChange={e =>
                      setSettings({ ...settings, docker: { ...settings.docker!, networkName: e.target.value } })
                    } placeholder="stroppy-net" />
                  </div>
                  <div className="space-y-1.5">
                    <Label className="text-xs">Edge Worker Image</Label>
                    <Input value={settings.docker?.edgeWorkerImage ?? ""} onChange={e =>
                      setSettings({ ...settings, docker: { ...settings.docker!, edgeWorkerImage: e.target.value } })
                    } placeholder="stroppy/edge-worker:latest" />
                  </div>
                </div>
                <Separator />
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-1.5">
                    <Label className="text-xs">Network CIDR</Label>
                    <Input value={settings.docker?.networkCidr ?? ""} placeholder="172.28.0.0/16" onChange={e =>
                      setSettings({ ...settings, docker: { ...settings.docker!, networkCidr: e.target.value } })
                    } />
                  </div>
                  <div className="space-y-1.5">
                    <Label className="text-xs">Network Prefix</Label>
                    <Input type="number" value={settings.docker?.networkPrefix ?? 24} onChange={e =>
                      setSettings({ ...settings, docker: { ...settings.docker!, networkPrefix: parseInt(e.target.value) || 24 } })
                    } />
                  </div>
                </div>
              </CardContent>
            </Card>
          )}

          {active === "yandex" && (
            <Card>
              <CardContent className="pt-6 pb-5 space-y-6">
                <div>
                  <h3 className="text-sm font-medium mb-1">Yandex Cloud</h3>
                  <p className="text-xs text-muted-foreground">Cloud provider settings (required for TARGET_YANDEX_CLOUD)</p>
                </div>
                <Separator />
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-1.5">
                    <Label className="text-xs">Cloud ID</Label>
                    <Input value={settings.yandexCloud?.providerSettings?.cloudId ?? ""} onChange={e =>
                      setSettings({ ...settings, yandexCloud: {
                        ...settings.yandexCloud!,
                        providerSettings: { ...settings.yandexCloud?.providerSettings!, cloudId: e.target.value }
                      }})
                    } />
                  </div>
                  <div className="space-y-1.5">
                    <Label className="text-xs">Folder ID</Label>
                    <Input value={settings.yandexCloud?.providerSettings?.folderId ?? ""} onChange={e =>
                      setSettings({ ...settings, yandexCloud: {
                        ...settings.yandexCloud!,
                        providerSettings: { ...settings.yandexCloud?.providerSettings!, folderId: e.target.value }
                      }})
                    } />
                  </div>
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-1.5">
                    <Label className="text-xs">Zone</Label>
                    <Input value={settings.yandexCloud?.providerSettings?.zone ?? ""} placeholder="ru-central1-a" onChange={e =>
                      setSettings({ ...settings, yandexCloud: {
                        ...settings.yandexCloud!,
                        providerSettings: { ...settings.yandexCloud?.providerSettings!, zone: e.target.value }
                      }})
                    } />
                  </div>
                  <div className="space-y-1.5">
                    <Label className="text-xs">Token</Label>
                    <Input type="password" value={settings.yandexCloud?.providerSettings?.token ?? ""} onChange={e =>
                      setSettings({ ...settings, yandexCloud: {
                        ...settings.yandexCloud!,
                        providerSettings: { ...settings.yandexCloud?.providerSettings!, token: e.target.value }
                      }})
                    } placeholder="••••••••" />
                  </div>
                </div>
                <Separator />
                <div className="space-y-1.5">
                  <Label className="text-xs">Base Image ID</Label>
                  <Input value={settings.yandexCloud?.vmSettings?.baseImageId ?? ""} onChange={e =>
                    setSettings({ ...settings, yandexCloud: {
                      ...settings.yandexCloud!,
                      vmSettings: { ...settings.yandexCloud?.vmSettings!, baseImageId: e.target.value }
                    }})
                  } />
                </div>
              </CardContent>
            </Card>
          )}
        </div>
      </div>
    </div>
  )
}
