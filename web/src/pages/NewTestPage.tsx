import { useState, useEffect, useCallback } from "react"
import { useNavigate } from "react-router-dom"
import { api } from "@/lib/api"
import { Card, CardContent } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Badge } from "@/components/ui/badge"
import { Separator } from "@/components/ui/separator"
import {
  Play, Loader2, ChevronLeft, ChevronRight, Plus, Trash2,
  Server, Database, Cpu, HardDrive, MemoryStick, Check
} from "lucide-react"
import type { Workload, TopologyTemplate } from "@/proto/api/api_pb.ts"
import { create } from "@bufbuild/protobuf"
import {
  TestSuiteSchema, TestSchema, StroppyCliSchema, type Test
} from "@/proto/stroppy/test_pb.ts"
import { HardwareSchema } from "@/proto/deployment/deployment_pb.ts"

const steps = [
  { id: "tests", label: "Tests", desc: "Define test cases" },
  { id: "workload", label: "Workload", desc: "Select benchmark" },
  { id: "topology", label: "Topology", desc: "Database topology" },
  { id: "hardware", label: "Hardware", desc: "Resource allocation" },
] as const

export function NewTestPage() {
  const navigate = useNavigate()
  const [workloads, setWorkloads] = useState<Workload[]>([])
  const [templates, setTemplates] = useState<TopologyTemplate[]>([])
  const [submitting, setSubmitting] = useState(false)
  const [step, setStep] = useState(0)

  const [tests, setTests] = useState<Test[]>([
    create(TestSchema, {
      name: "test-1",
      stroppyCli: create(StroppyCliSchema, { version: "v1.0.0" }),
      stroppyHardware: create(HardwareSchema, { cores: 2, memory: 4, disk: 20 }),
    })
  ])

  const [selectedWorkloads, setSelectedWorkloads] = useState<(string | null)[]>([null])
  const [selectedTemplates, setSelectedTemplates] = useState<(string | null)[]>([null])
  const [activeTestIdx, setActiveTestIdx] = useState(0)

  const loadData = useCallback(async () => {
    const [wRes, tRes] = await Promise.all([
      api.listWorkloads({ limit: BigInt(100) }).catch(() => ({ workloads: [], total: BigInt(0) })),
      api.listTopologyTemplates({ limit: BigInt(100) }).catch(() => ({ topologyTemplates: [], total: BigInt(0) })),
    ])
    setWorkloads(wRes.workloads)
    setTemplates(tRes.topologyTemplates)
  }, [])

  useEffect(() => { loadData() }, [loadData])

  const activeTest = tests[activeTestIdx]

  const updateTest = (idx: number, updates: Partial<Test>) => {
    setTests(prev => prev.map((t, i) => i === idx ? { ...t, ...updates } as Test : t))
  }

  const addTest = () => {
    const t = create(TestSchema, {
      name: `test-${tests.length + 1}`,
      stroppyCli: create(StroppyCliSchema, { version: "v1.0.0" }),
      stroppyHardware: create(HardwareSchema, { cores: 2, memory: 4, disk: 20 }),
    })
    setTests(prev => [...prev, t])
    setSelectedWorkloads(prev => [...prev, null])
    setSelectedTemplates(prev => [...prev, null])
    setActiveTestIdx(tests.length)
  }

  const removeTest = (idx: number) => {
    if (tests.length <= 1) return
    setTests(prev => prev.filter((_, i) => i !== idx))
    setSelectedWorkloads(prev => prev.filter((_, i) => i !== idx))
    setSelectedTemplates(prev => prev.filter((_, i) => i !== idx))
    setActiveTestIdx(prev => Math.min(prev, tests.length - 2))
  }

  const selectWorkload = (idx: number, workloadId: string) => {
    setSelectedWorkloads(prev => prev.map((w, i) => i === idx ? workloadId : w))
  }

  const selectTemplate = (idx: number, templateId: string) => {
    const tmpl = templates.find(t => t.id === templateId)
    setSelectedTemplates(prev => prev.map((t, i) => i === idx ? templateId : t))
    if (tmpl?.template) {
      updateTest(idx, {
        databaseRef: { case: "databaseTemplate" as const, value: tmpl.template }
      })
    }
  }

  const handleRun = async () => {
    setSubmitting(true)
    try {
      const suite = create(TestSuiteSchema, { tests })
      const settingsRes = await api.getSettings({})
      const res = await api.runTestSuite({
        suite,
        settings: settingsRes.settings,
        target: settingsRes.settings?.preferredTarget ?? 0,
      })
      navigate(`/suites/${res.suiteId}`)
    } catch (err) {
      alert(err instanceof Error ? err.message : "Failed to run test suite")
    }
    setSubmitting(false)
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => navigate(-1)}>
            <ChevronLeft className="w-4 h-4" />
          </Button>
          <div>
            <h1 className="text-xl font-semibold tracking-tight">New Test</h1>
            <p className="text-muted-foreground text-sm mt-0.5">Configure and run a test suite</p>
          </div>
        </div>
      </div>

      {/* Stepper indicator */}
      <div className="flex items-center gap-2">
        {steps.map((s, i) => (
          <div key={s.id} className="flex items-center gap-2 flex-1">
            <button onClick={() => setStep(i)}
              className={`flex items-center gap-2.5 px-3 py-2 rounded-lg transition-all duration-150 w-full ${
                i === step
                  ? "bg-primary/10 border border-primary/30"
                  : i < step
                    ? "bg-muted/40 border border-border/60"
                    : "border border-transparent"
              }`}
            >
              <div className={`w-7 h-7 rounded-full flex items-center justify-center text-[11px] font-mono font-semibold shrink-0 ${
                i < step
                  ? "bg-primary text-primary-foreground"
                  : i === step
                    ? "bg-primary/20 text-primary border border-primary/30"
                    : "bg-muted text-muted-foreground"
              }`}>
                {i < step ? <Check className="w-3.5 h-3.5" /> : i + 1}
              </div>
              <div className="min-w-0 text-left">
                <div className={`text-[12px] font-medium truncate ${i === step ? "text-primary" : i < step ? "text-foreground" : "text-muted-foreground"}`}>{s.label}</div>
                <div className="text-[10px] text-muted-foreground truncate">{s.desc}</div>
              </div>
            </button>
            {i < steps.length - 1 && (
              <div className={`w-8 h-px shrink-0 ${i < step ? "bg-primary/40" : "bg-border/60"}`} />
            )}
          </div>
        ))}
      </div>

      {/* Step content */}
      <Card>
        <CardContent className="pt-6 pb-5">
          {/* Step 0: Tests */}
          {step === 0 && (
            <div className="space-y-4">
              <div className="flex items-center justify-between mb-2">
                <div>
                  <h3 className="text-sm font-medium">Test Cases</h3>
                  <p className="text-xs text-muted-foreground mt-0.5">Add and configure individual tests in your suite</p>
                </div>
                <Button variant="outline" size="sm" onClick={addTest}>
                  <Plus className="w-3.5 h-3.5 mr-1.5" />Add Test
                </Button>
              </div>
              <div className="space-y-3">
                {tests.map((t, i) => (
                  <div key={i}
                    className={`p-4 rounded-xl border transition-all duration-150 cursor-pointer ${
                      i === activeTestIdx ? "border-primary/40 bg-primary/5" : "hover:border-border/80 hover:bg-muted/30"
                    }`}
                    onClick={() => setActiveTestIdx(i)}
                  >
                    <div className="flex items-center justify-between mb-3">
                      <div className="flex items-center gap-2">
                        <div className={`w-6 h-6 rounded-md flex items-center justify-center text-[10px] font-mono font-semibold ${
                          i === activeTestIdx ? "bg-primary/20 text-primary" : "bg-muted text-muted-foreground"
                        }`}>{i + 1}</div>
                        <span className="font-medium text-sm">{t.name || `test-${i + 1}`}</span>
                      </div>
                      {tests.length > 1 && (
                        <Button variant="ghost" size="icon" className="h-6 w-6" onClick={(e) => { e.stopPropagation(); removeTest(i) }}>
                          <Trash2 className="w-3 h-3" />
                        </Button>
                      )}
                    </div>
                    {i === activeTestIdx && (
                      <div className="grid grid-cols-2 gap-4 mt-3 pt-3 border-t border-border/40">
                        <div className="space-y-1.5">
                          <Label className="text-xs">Test Name</Label>
                          <Input value={activeTest.name} onClick={e => e.stopPropagation()} onChange={e => updateTest(activeTestIdx, { name: e.target.value })} />
                        </div>
                        <div className="space-y-1.5">
                          <Label className="text-xs">Description</Label>
                          <Input value={activeTest.description ?? ""} onClick={e => e.stopPropagation()} onChange={e => updateTest(activeTestIdx, { description: e.target.value || undefined })} placeholder="Optional" />
                        </div>
                      </div>
                    )}
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Step 1: Workload */}
          {step === 1 && (
            <div className="space-y-4">
              <div>
                <h3 className="text-sm font-medium">Select Workload</h3>
                <p className="text-xs text-muted-foreground mt-0.5">Choose a registered test workload for <span className="font-mono font-medium text-foreground">{activeTest.name}</span></p>
              </div>
              {workloads.length === 0 ? (
                <p className="text-sm text-muted-foreground py-8 text-center">No workloads registered. Go to Workloads page to upload one.</p>
              ) : (
                <div className="grid grid-cols-2 gap-3">
                  {workloads.map(w => (
                    <button key={w.id}
                      onClick={() => selectWorkload(activeTestIdx, w.id)}
                      className={`p-3.5 rounded-xl border text-left transition-all duration-150 ${
                        selectedWorkloads[activeTestIdx] === w.id
                          ? "border-primary bg-primary/5 shadow-sm"
                          : "hover:bg-muted/50 hover:border-border/80"
                      }`}
                    >
                      <div className="flex items-center gap-2 mb-1.5">
                        <div className="w-6 h-6 rounded-md bg-muted flex items-center justify-center shrink-0">
                          <Database className="w-3 h-3 text-muted-foreground" />
                        </div>
                        <span className="font-medium text-sm">{w.name}</span>
                        {w.builtin && <Badge variant="outline" className="text-[9px] font-mono">builtin</Badge>}
                      </div>
                      {w.description && <p className="text-[11px] text-muted-foreground ml-8">{w.description}</p>}
                      <div className="flex gap-1.5 mt-2 ml-8 flex-wrap">
                        {w.probe?.steps.map((s, i) => (
                          <Badge key={i} variant="secondary" className="text-[9px] font-mono">{s}</Badge>
                        ))}
                      </div>
                    </button>
                  ))}
                </div>
              )}
            </div>
          )}

          {/* Step 2: Topology */}
          {step === 2 && (
            <div className="space-y-4">
              <div>
                <h3 className="text-sm font-medium">Select Topology</h3>
                <p className="text-xs text-muted-foreground mt-0.5">Choose a database topology template or use a connection string</p>
              </div>
              <div className="grid grid-cols-2 gap-3">
                {templates.map(t => (
                  <button key={t.id}
                    onClick={() => selectTemplate(activeTestIdx, t.id)}
                    className={`p-3.5 rounded-xl border text-left transition-all duration-150 ${
                      selectedTemplates[activeTestIdx] === t.id
                        ? "border-primary bg-primary/5 shadow-sm"
                        : "hover:bg-muted/50 hover:border-border/80"
                    }`}
                  >
                    <div className="flex items-center gap-2 mb-1.5">
                      <div className="w-6 h-6 rounded-md bg-muted flex items-center justify-center shrink-0">
                        <Server className="w-3 h-3 text-muted-foreground" />
                      </div>
                      <span className="font-medium text-sm">{t.name}</span>
                      {t.builtin && <Badge variant="outline" className="text-[9px] font-mono">builtin</Badge>}
                    </div>
                    {t.description && <p className="text-[11px] text-muted-foreground ml-8">{t.description}</p>}
                    <Badge variant="secondary" className="mt-2 ml-8 text-[9px] font-mono">
                      {t.databaseType === 1 ? "PostgreSQL" : t.databaseType === 2 ? "Picodata" : "Unknown"}
                    </Badge>
                  </button>
                ))}
              </div>
              <Separator />
              <div className="space-y-1.5">
                <Label className="text-xs">Or use connection string</Label>
                <Input placeholder="postgresql://user:pass@host:5432/db" className="font-mono text-xs" onChange={e => {
                  if (e.target.value) {
                    updateTest(activeTestIdx, {
                      databaseRef: { case: "connectionString" as const, value: e.target.value }
                    })
                    setSelectedTemplates(prev => prev.map((t, i) => i === activeTestIdx ? null : t))
                  }
                }} />
              </div>
            </div>
          )}

          {/* Step 3: Hardware */}
          {step === 3 && (
            <div className="space-y-4">
              <div>
                <h3 className="text-sm font-medium">Stroppy Hardware</h3>
                <p className="text-xs text-muted-foreground mt-0.5">Resources for the stroppy test runner VM</p>
              </div>
              <div className="grid grid-cols-3 gap-6">
                {[
                  { label: "CPU Cores", icon: Cpu, key: "cores" as const, value: activeTest.stroppyHardware?.cores ?? 2, min: 1 },
                  { label: "Memory (GB)", icon: MemoryStick, key: "memory" as const, value: activeTest.stroppyHardware?.memory ?? 4, min: 1 },
                  { label: "Disk (GB)", icon: HardDrive, key: "disk" as const, value: activeTest.stroppyHardware?.disk ?? 20, min: 1 },
                ].map(hw => (
                  <div key={hw.key} className="p-4 rounded-xl border border-border/60 space-y-3">
                    <div className="flex items-center gap-2">
                      <div className="w-8 h-8 rounded-lg bg-muted/60 flex items-center justify-center">
                        <hw.icon className="w-4 h-4 text-muted-foreground" />
                      </div>
                      <span className="text-xs font-medium">{hw.label}</span>
                    </div>
                    <Input type="number" min={hw.min} value={hw.value}
                      className="font-mono text-lg font-semibold h-12 text-center"
                      onChange={e => updateTest(activeTestIdx, {
                        stroppyHardware: { ...activeTest.stroppyHardware!, [hw.key]: parseInt(e.target.value) || hw.min }
                      })} />
                  </div>
                ))}
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Navigation */}
      <div className="flex items-center justify-between">
        <Button variant="outline" size="sm" onClick={() => setStep(s => s - 1)} disabled={step === 0}>
          <ChevronLeft className="w-3.5 h-3.5 mr-1.5" />Previous
        </Button>
        <div className="text-xs text-muted-foreground font-mono">
          Step {step + 1} of {steps.length}
        </div>
        {step < steps.length - 1 ? (
          <Button size="sm" onClick={() => setStep(s => s + 1)}>
            Next<ChevronRight className="w-3.5 h-3.5 ml-1.5" />
          </Button>
        ) : (
          <Button onClick={handleRun} disabled={submitting} size="sm">
            {submitting ? <Loader2 className="w-3.5 h-3.5 mr-1.5 animate-spin" /> : <Play className="w-3.5 h-3.5 mr-1.5" />}
            Run Suite
          </Button>
        )}
      </div>
    </div>
  )
}
