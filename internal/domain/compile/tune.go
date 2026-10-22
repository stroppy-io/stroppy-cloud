package compile

import (
	"encoding/json"
	"math"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// The tuning of a database (db.<kind>.params tuning): "auto" sizes the
// engine's settings from the machine it lands on, its role, the topology
// and the workload; "stock" renders the schema defaults (the engine's own).
// A key the database sets in a role's config always wins over auto.
const (
	tuningAuto  = "auto"
	tuningStock = "stock"
)

// tuning is the database's tuning mode. Before the common field MySQL and
// MariaDB had buffer_pool: auto|fixed|stock; stock there still means stock.
func (c *compilation) tuning() string {
	p := c.params()
	if v := strParam(p, "tuning", ""); v != "" {
		return v
	}
	if strParam(p, "buffer_pool", "") == tuningStock {
		return tuningStock
	}
	return tuningAuto
}

// host is what the tuning knows about one machine.
type host struct {
	CPU      int
	MemoryMB int
	DiskGB   int
	DiskType string
	Disk     catalog.DiskLimits
}

// hostOf describes a machine of the run (zero when unknown).
func (c *compilation) hostOf(machine string) host {
	for _, m := range c.out.Spec.Machines {
		if m.Name != machine {
			continue
		}
		h := host{CPU: m.CPU, MemoryMB: m.MemoryGB * 1024}
		for _, d := range m.Disks {
			if d.Mount == dataMount {
				h.DiskGB, h.DiskType = d.GB, d.Type
				h.Disk = c.in.Provider.DiskLimitsOf(d.Type, d.GB, m.CPU)
			}
		}
		return h
	}
	return host{}
}

// defaultVUs stands in for a workload whose segments name no VU count.
const defaultVUs = 128

// peakVUs is the most virtual users one segment of the workload runs at
// once: what the database's connection limits must take.
func (c *compilation) peakVUs() int {
	peak := 0
	for _, raw := range c.in.Workload.Segments {
		var s spec.Segment
		if json.Unmarshal(raw, &s) == nil {
			peak = max(peak, int(s.Run.VUs))
		}
	}
	if peak == 0 {
		return defaultVUs
	}
	return peak
}

// tuner applies auto values to one role/schema overlay: a key the
// database set explicitly is kept, every other one is set and recorded,
// so the run's snapshot shows what the database really got.
type tuner struct {
	c        *compilation
	role     string
	schemaID string
	explicit map[string]bool
	overlay  map[string]any
}

// tune starts the auto values of a role's config. The overlay is the one
// passed to render.
func (c *compilation) tune(role, schemaID string, overlay map[string]any) tuner {
	return tuner{c: c, role: role, schemaID: schemaID, explicit: c.explicitConfig(role, schemaID), overlay: overlay}
}

// set puts an auto value unless the database set the key itself.
func (t tuner) set(key string, value any) {
	if t.explicit[key] {
		return
	}
	t.overlay[key] = value
	t.c.recordTuned(t.role, t.schemaID, key, value)
}

// has reports a key the database set itself.
func (t tuner) has(key string) bool { return t.explicit[key] }

// recordTuned keeps an auto value for the run's effective configs.
func (c *compilation) recordTuned(role, schemaID, key string, value any) {
	if c.out.Tuned == nil {
		c.out.Tuned = map[string]map[string]map[string]any{}
	}
	if c.out.Tuned[role] == nil {
		c.out.Tuned[role] = map[string]map[string]any{}
	}
	if c.out.Tuned[role][schemaID] == nil {
		c.out.Tuned[role][schemaID] = map[string]any{}
	}
	c.out.Tuned[role][schemaID][key] = value
}

// EffectiveWithTuned is the run's effective configs: the database's
// resolved values with the auto values the compiler put on top.
func EffectiveWithTuned(base map[string]map[string]json.RawMessage, tuned map[string]map[string]map[string]any) map[string]map[string]json.RawMessage {
	if len(tuned) == 0 {
		return base
	}
	out := make(map[string]map[string]json.RawMessage, len(base))
	for role, byID := range base {
		out[role] = make(map[string]json.RawMessage, len(byID))
		for id, v := range byID {
			out[role][id] = v
		}
	}
	for role, byID := range tuned {
		if out[role] == nil {
			out[role] = map[string]json.RawMessage{}
		}
		for id, keys := range byID {
			value := map[string]any{}
			if raw, ok := out[role][id]; ok {
				_ = json.Unmarshal(raw, &value) //nolint:errcheck // baked upstream
			}
			for k, v := range keys {
				value[k] = v
			}
			if raw, err := json.Marshal(value); err == nil {
				out[role][id] = raw
			}
		}
	}
	return out
}

// share is a fraction of an amount, rounded down to a step.
func share(total int, frac float64, step int) int {
	v := int(float64(total) * frac)
	if step > 1 {
		v -= v % step
	}
	return v
}

// clamp bounds v to [lo, hi].
func clamp(v, lo, hi int) int { return max(lo, min(v, hi)) }

// ceilTo rounds v up to a multiple of step.
func ceilTo(v, step int) int { return int(math.Ceil(float64(v)/float64(step))) * step }
