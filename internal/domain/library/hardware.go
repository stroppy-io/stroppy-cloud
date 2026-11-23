package library

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
	pipelinespec "github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// DataDiskGB is the storage one machine of a role needs for the workload:
// the recommended budget times the share of the dataset the plan puts on
// that machine. 0 for a role that stores no table data.
func DataDiskGB(plan topology.Plan, role string, estimate pipelinespec.DatasetEstimate) int {
	return int(math.Ceil(float64(estimate.RecommendedGB) * plan.DataShare[role]))
}

// ResolveHardware is shared by the fit preview and recipe compiler. A preset is
// a base, not a limit; omitted storage grows to dataGB, the machine's share
// of the workload planning budget (DataDiskGB).
func ResolveHardware(base pipelinespec.Machine, rs RoleSize, dataGB int, runtime ...json.RawMessage) (pipelinespec.Machine, error) {
	base.Disks = slices.Clone(base.Disks)
	if len(base.Disks) > 0 && rs.DiskGB == 0 {
		base.Disks[0].GB = max(base.Disks[0].GB, dataGB)
	}
	if len(base.Disks) > 0 {
		if rs.DiskGB > 0 {
			base.Disks[0].GB = rs.DiskGB
		}
		if rs.DiskType != "" {
			base.Disks[0].Type = rs.DiskType
		}
		if rs.DiskGB == 0 && (base.Disks[0].Type == "network-ssd-nonreplicated" || base.Disks[0].Type == "network-ssd-io-m3") {
			base.Disks[0].GB = ((base.Disks[0].GB + 92) / 93) * 93
		}
	}
	if len(rs.Machine) > 0 {
		raw, err := pipelinespec.NormalizeMachineOverride(rs.Machine)
		if err != nil {
			return base, err
		}
		if err := pipelinespec.Overlay(base, raw, &base); err != nil {
			return base, err
		}
	}
	for _, raw := range runtime {
		if len(raw) == 0 {
			continue
		}
		value, err := pipelinespec.NormalizeRuntime(raw)
		if err != nil {
			return base, err
		}
		var overlay pipelinespec.Runtime
		if err := json.Unmarshal(value, &overlay); err != nil {
			return base, err
		}
		if patch, ok := overlay.Machines[base.Name]; ok {
			if err := pipelinespec.Overlay(base, patch, &base); err != nil {
				return base, err
			}
		}
	}
	if base.CPU < 1 || base.MemoryGB < 1 {
		return base, fmt.Errorf("hardware requires positive CPU and RAM")
	}
	return base, nil
}

func previewHardware(role string, index int, rs RoleSize, cell catalog.SizeSpec, dataGB int, runtime ...json.RawMessage) (pipelinespec.Machine, error) {
	m := pipelinespec.Machine{Name: fmt.Sprintf("%s-%d", role, index), Role: role, CPU: cell.CPU, MemoryGB: cell.MemoryGB, InstanceType: cell.InstanceType}
	if role != "runner" && role != "proxy" {
		m.Disks = []pipelinespec.Disk{{Name: "data", GB: cell.DefaultDiskGB, Type: cell.DiskType, Mount: "/data"}}
	}
	return ResolveHardware(m, rs, dataGB, runtime...)
}
