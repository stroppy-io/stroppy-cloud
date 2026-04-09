import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

const SLIDER_TRACK = "w-full h-1.5 bg-zinc-800 rounded-full appearance-none cursor-pointer accent-primary disabled:opacity-50 [&::-webkit-slider-thumb]:w-3 [&::-webkit-slider-thumb]:h-3 [&::-webkit-slider-thumb]:rounded-full [&::-webkit-slider-thumb]:bg-primary [&::-webkit-slider-thumb]:appearance-none";

export function closestStep(val: number, steps: number[]): number {
  let best = steps[0];
  for (const s of steps) {
    if (Math.abs(s - val) < Math.abs(best - val)) best = s;
  }
  return best;
}

export function SliderField({ label, value, steps, onChange, disabled, format }: {
  label: string;
  value: number;
  steps: number[];
  onChange: (v: number) => void;
  disabled?: boolean;
  format?: (v: number) => string;
}) {
  const idx = steps.indexOf(closestStep(value, steps));
  return (
    <div className="space-y-1">
      <Label className="text-[9px] font-mono text-zinc-600">{label}</Label>
      <div className="flex items-center gap-2">
        <input type="range" min={0} max={steps.length - 1} value={idx >= 0 ? idx : 0}
          onChange={(e) => onChange(steps[parseInt(e.target.value)])}
          disabled={disabled}
          className={SLIDER_TRACK + " flex-1"} />
        <Input value={format ? format(value) : String(value)}
          onChange={(e) => {
            const n = parseInt(e.target.value.replace(/[^\d]/g, ""));
            if (!isNaN(n) && n >= steps[0]) onChange(closestStep(n, steps));
          }}
          className="h-6 w-16 text-[10px] font-mono text-right tabular-nums shrink-0" disabled={disabled} />
      </div>
    </div>
  );
}

export function NumericSlider({ label, value, min, max, step, onChange, disabled, hint }: {
  label: string;
  value: number;
  min: number;
  max: number;
  step?: number;
  onChange: (v: number) => void;
  disabled?: boolean;
  hint?: string;
}) {
  return (
    <div className="space-y-1">
      <Label className="text-[9px] font-mono text-zinc-600">{label}</Label>
      <div className="flex items-center gap-2">
        <input type="range" min={min} max={max} step={step || 1} value={value}
          onChange={(e) => onChange(parseInt(e.target.value))}
          disabled={disabled}
          className={SLIDER_TRACK + " flex-1"} />
        <Input type="number" min={min} value={value}
          onChange={(e) => {
            const n = parseInt(e.target.value);
            if (!isNaN(n) && n >= min) onChange(n);
          }}
          className="h-6 w-16 text-[10px] font-mono text-right tabular-nums shrink-0" disabled={disabled} />
      </div>
      {hint && <span className="text-[9px] text-zinc-700 font-mono">{hint}</span>}
    </div>
  );
}

// ─── Duration Slider ─────────────────────────────────────────────

const DURATION_STEPS = ["1m", "2m", "5m", "10m", "15m", "30m", "1h", "2h", "4h", "8h", "12h", "24h"];

function parseDuration(s: string): string {
  const trimmed = s.trim().toLowerCase();
  if (/^\d+[smh]$/.test(trimmed)) return trimmed;
  if (/^\d+$/.test(trimmed)) return trimmed + "m";
  return trimmed || "5m";
}

export function DurationSlider({ label, value, onChange, disabled }: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  disabled?: boolean;
}) {
  const normalized = parseDuration(value);
  const idx = DURATION_STEPS.indexOf(normalized);

  return (
    <div className="space-y-1">
      <Label className="text-[9px] font-mono text-zinc-600">{label}</Label>
      <div className="flex items-center gap-2">
        <input type="range" min={0} max={DURATION_STEPS.length - 1} value={idx >= 0 ? idx : 0}
          onChange={(e) => onChange(DURATION_STEPS[parseInt(e.target.value)])}
          disabled={disabled}
          className={SLIDER_TRACK + " flex-1"} />
        <Input value={value}
          onChange={(e) => onChange(e.target.value)}
          onBlur={(e) => onChange(parseDuration(e.target.value))}
          className="h-6 w-16 text-[10px] font-mono text-right tabular-nums shrink-0" disabled={disabled} />
      </div>
    </div>
  );
}

export const CPU_STEPS = [2, 4, 8, 12, 16, 24, 32];
export const DISK_STEPS = [25, 50, 100, 200, 300, 500, 750, 1024];

/**
 * Yandex Cloud disk performance specs.
 * Source: https://yandex.cloud/en/docs/compute/concepts/limits
 * Performance scales by allocation units: units = ceil(disk_size / unitGb).
 */
const DISK_SPECS: Record<string, {
  label: string;
  unitGb: number;
  readIopsPerUnit: number; maxReadIops: number;
  writeIopsPerUnit: number; maxWriteIops: number;
  readMbPerUnit: number; maxReadMb: number;
  writeMbPerUnit: number; maxWriteMb: number;
}> = {
  "network-ssd": {
    label: "SSD",
    unitGb: 32,
    readIopsPerUnit: 1000, maxReadIops: 20000,
    writeIopsPerUnit: 1000, maxWriteIops: 40000,
    readMbPerUnit: 15, maxReadMb: 450,
    writeMbPerUnit: 15, maxWriteMb: 450,
  },
  "network-ssd-io-m3": {
    label: "SSD io-m3",
    unitGb: 32,
    readIopsPerUnit: 28000, maxReadIops: 75000,
    writeIopsPerUnit: 5600, maxWriteIops: 40000,
    readMbPerUnit: 110, maxReadMb: 1024,
    writeMbPerUnit: 82, maxWriteMb: 1024,
  },
};

function calcDiskPerf(type: string, sizeGb: number) {
  const s = DISK_SPECS[type] || DISK_SPECS["network-ssd"];
  const units = Math.max(1, Math.ceil(sizeGb / s.unitGb));
  return {
    readIops: Math.min(s.maxReadIops, s.readIopsPerUnit * units),
    writeIops: Math.min(s.maxWriteIops, s.writeIopsPerUnit * units),
    readMb: Math.min(s.maxReadMb, s.readMbPerUnit * units),
    writeMb: Math.min(s.maxWriteMb, s.writeMbPerUnit * units),
  };
}

function fmtK(n: number): string {
  return n >= 1000 ? `${(n / 1000).toFixed(n % 1000 === 0 ? 0 : 1)}K` : String(n);
}

export function DiskTypeSelect({ value, onChange, diskSizeGb }: { value: string; onChange: (v: string) => void; diskSizeGb?: number }) {
  const size = diskSizeGb || 50;
  return (
    <div>
      <label className="text-[10px] font-mono text-zinc-500 uppercase tracking-wider mb-1 block">Disk Type</label>
      <div className="flex gap-1.5">
        {Object.entries(DISK_SPECS).map(([id, spec]) => {
          const perf = calcDiskPerf(id, size);
          const active = value === id;
          return (
            <button
              key={id}
              type="button"
              onClick={() => onChange(id)}
              className={`flex-1 px-2 py-1.5 text-left border transition-colors ${
                active
                  ? "border-primary/40 bg-primary/[0.06]"
                  : "border-zinc-800 hover:border-zinc-700"
              }`}
            >
              <div className={`text-[11px] font-mono font-medium ${active ? "text-primary" : "text-zinc-400"}`}>{spec.label}</div>
              <div className="text-[9px] text-zinc-600">
                {fmtK(perf.readIops)} / {fmtK(perf.writeIops)} IOPS
              </div>
              <div className="text-[9px] text-zinc-700">
                {perf.readMb} / {perf.writeMb} MB/s
              </div>
            </button>
          );
        })}
      </div>
    </div>
  );
}

export function ramSteps(cpus: number): number[] {
  const min = cpus * 1024;
  const steps: number[] = [];
  let v = min;
  while (v <= 262144) {
    steps.push(v);
    if (v < 8192) v += 1024;
    else if (v < 32768) v += 4096;
    else if (v < 65536) v += 8192;
    else v += 32768;
  }
  return steps;
}
