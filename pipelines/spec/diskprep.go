package spec

import (
	_ "embed"
	"strings"
)

//go:embed mount-data.sh
var mountDataScript string

// EnsureDiskPreparation handles mounted YC disks for both compiled recipes and
// direct RunSpec callers. Exact generated steps are deduplicated on replay.
func EnsureDiskPreparation(r *Run) {
	if r.Provider.Kind != ProviderYandex {
		return
	}
	var prep []HostPrep
	for _, m := range r.Machines {
		for _, d := range m.Disks {
			if d.Mount == "" {
				continue
			}
			fs := d.Filesystem
			if fs == "" {
				fs = "ext4"
			}
			options := "defaults"
			if len(d.MountOptions) > 0 {
				options = strings.Join(d.MountOptions, ",")
			}
			content := strings.NewReplacer("device=/dev/disk/by-id/virtio-data", "device="+shellQuote("/dev/disk/by-id/virtio-"+d.Name), "target=/data", "target="+shellQuote(d.Mount), "filesystem=ext4", "filesystem="+shellQuote(fs), "options=defaults", "options="+shellQuote(options)).Replace(mountDataScript)
			found := false
			for _, h := range r.HostPrep {
				if h.Role == m.Role && h.Machine == m.Name && h.Kind == HostPrepScript && h.Content == content {
					found = true
					break
				}
			}
			if !found {
				prep = append(prep, HostPrep{Role: m.Role, Machine: m.Name, Kind: HostPrepScript, Content: content})
			}
		}
	}
	r.HostPrep = append(prep, r.HostPrep...)
}
func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
