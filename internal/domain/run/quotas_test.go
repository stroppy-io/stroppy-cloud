package run

import (
	"context"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/provider"
	pipelinespec "github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

type quotaProfiles struct {
	Profiles
	quota provider.Quota
}

func (p quotaProfiles) FreshQuotas(context.Context, provider.Profile) ([]provider.Quota, error) {
	return []provider.Quota{p.quota}, nil
}

func TestQuotasUseFinalMachineResources(t *testing.T) {
	input := pipelinespec.Run{
		Provider: pipelinespec.Provider{Kind: pipelinespec.ProviderYandex},
		Network:  pipelinespec.Network{AllowPublicIPs: true},
		Machines: []pipelinespec.Machine{
			{CPU: 8, MemoryGB: 32, InstanceType: "standard-v3", BootDisk: &pipelinespec.BootDisk{GB: 80, Type: "network-ssd"}, Disks: []pipelinespec.Disk{{GB: 186, Type: "network-ssd-io-m3"}}},
			{CPU: 2, MemoryGB: 8, InstanceType: "highfreq-v3"},
		},
	}
	for _, tc := range []struct {
		name      string
		quota     provider.Quota
		wantError bool
	}{
		{"all cores", provider.Quota{Name: "compute.instanceCores.count", Limit: 9}, true},
		{"memory bytes", provider.Quota{Name: "compute.instanceMemory.size", Limit: 39 * (1 << 30), Unit: "bytes"}, true},
		{"both boot disks", provider.Quota{Name: "compute.ssdDisks.size", Limit: 119, Unit: "GiB"}, true},
		{"disk count", provider.Quota{Name: "compute.disks.count", Limit: 2}, true},
		{"io-m3 bytes", provider.Quota{Name: "compute.ssdIOM3Disks.size", Limit: 185 * (1 << 30), Unit: "bytes"}, true},
		{"io-m3 used", provider.Quota{Name: "compute.ssdIOM3Disks.size", Limit: 200, Used: 15, Unit: "GiB"}, true},
		{"io-m3 zero", provider.Quota{Name: "compute.ssdIOM3Disks.size", Limit: 0}, true},
		{"highfreq only", provider.Quota{Name: "compute.instanceHighFreqCores.count", Limit: 2}, false},
		{"highfreq exhausted", provider.Quota{Name: "compute.instanceHighFreqCores.count", Limit: 0}, true},
		{"unused class zero", provider.Quota{Name: "compute.hddDisks.size", Limit: 0}, false},
		{"exact capacity", provider.Quota{Name: "compute.ssdDisks.size", Limit: 120}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &Service{profiles: quotaProfiles{quota: tc.quota}}
			err := svc.checkQuotas(t.Context(), provider.Profile{}, input)
			if (err != nil) != tc.wantError {
				t.Fatalf("checkQuotas error = %v, want error %v", err, tc.wantError)
			}
		})
	}
	input.Machines = input.Machines[:1]
	svc := &Service{profiles: quotaProfiles{quota: provider.Quota{Name: "compute.instanceHighFreqCores.count", Limit: 0}}}
	if err := svc.checkQuotas(t.Context(), provider.Profile{}, input); err != nil {
		t.Fatalf("ordinary platform must not consume high-frequency quota: %v", err)
	}
}
