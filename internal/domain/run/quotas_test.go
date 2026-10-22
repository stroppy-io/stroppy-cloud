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
		name     string
		quota    provider.Quota
		short    bool // no room now
		never    bool // more than the whole limit
		launchOK bool
	}{
		{"all cores", provider.Quota{Name: "compute.instanceCores.count", Limit: 9}, true, true, false},
		{"memory bytes", provider.Quota{Name: "compute.instanceMemory.size", Limit: 39 * (1 << 30), Unit: "bytes"}, true, true, false},
		{"both boot disks", provider.Quota{Name: "compute.ssdDisks.size", Limit: 119, Unit: "GiB"}, true, true, false},
		{"disk count", provider.Quota{Name: "compute.disks.count", Limit: 2}, true, true, false},
		{"io-m3 bytes", provider.Quota{Name: "compute.ssdIOM3Disks.size", Limit: 185 * (1 << 30), Unit: "bytes"}, true, true, false},
		{"io-m3 used", provider.Quota{Name: "compute.ssdIOM3Disks.size", Limit: 200, Used: 15, Unit: "GiB"}, true, false, true},
		{"io-m3 zero", provider.Quota{Name: "compute.ssdIOM3Disks.size", Limit: 0}, true, true, false},
		{"highfreq only", provider.Quota{Name: "compute.instanceHighFreqCores.count", Limit: 2}, false, false, true},
		{"highfreq exhausted", provider.Quota{Name: "compute.instanceHighFreqCores.count", Limit: 0}, true, true, false},
		{"unused class zero", provider.Quota{Name: "compute.hddDisks.size", Limit: 0}, false, false, true},
		{"exact capacity", provider.Quota{Name: "compute.ssdDisks.size", Limit: 120}, false, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sf, never := quotaShortfall([]provider.Quota{tc.quota}, nil, input)
			if (sf != nil) != tc.short || never != tc.never {
				t.Fatalf("shortfall %v never %v, want %v %v", sf, never, tc.short, tc.never)
			}
			svc := &Service{profiles: quotaProfiles{quota: tc.quota}}
			if err := svc.checkQuotas(t.Context(), provider.Profile{}, input); (err == nil) != tc.launchOK {
				t.Fatalf("checkQuotas error = %v, want launch ok %v", err, tc.launchOK)
			}
		})
	}
	input.Machines = input.Machines[:1]
	if sf, _ := quotaShortfall([]provider.Quota{{Name: "compute.instanceHighFreqCores.count", Limit: 0}}, nil, input); sf != nil {
		t.Fatalf("ordinary platform must not consume high-frequency quota: %v", sf)
	}
}

func TestZonalQuotasCountOnlyTheirZone(t *testing.T) {
	input := pipelinespec.Run{
		Provider: pipelinespec.Provider{Kind: pipelinespec.ProviderYandex, Settings: []byte(`{"zone":"ru-central1-d"}`)},
		Network:  pipelinespec.Network{AllowPublicIPs: true},
		Machines: []pipelinespec.Machine{
			{CPU: 16, MemoryGB: 64, Disks: []pipelinespec.Disk{{GB: 400, Type: "network-ssd"}}},
			{CPU: 16, MemoryGB: 64, Location: "ru-central1-a", Disks: []pipelinespec.Disk{{GB: 400, Type: "network-ssd"}}},
		},
	}
	for _, tc := range []struct {
		name  string
		quota provider.Quota
		short bool
	}{
		{"default zone takes the unplaced machine", provider.Quota{Name: "compute.ssdDisks.size", Zone: "ru-central1-d", Limit: 1000, Used: 600, Unit: "GiB"}, true},
		{"default zone fits", provider.Quota{Name: "compute.ssdDisks.size", Zone: "ru-central1-d", Limit: 1000, Used: 560, Unit: "GiB"}, false},
		{"other zone counts its machine", provider.Quota{Name: "compute.instanceCores.count", Zone: "ru-central1-a", Limit: 20, Used: 5}, true},
		{"zone without machines", provider.Quota{Name: "compute.instanceCores.count", Zone: "ru-central1-b", Limit: 0}, false},
		{"cloud-wide counts both", provider.Quota{Name: "compute.instanceCores.count", Limit: 31}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if sf, _ := quotaShortfall([]provider.Quota{tc.quota}, nil, input); (sf != nil) != tc.short {
				t.Fatalf("shortfall %v, want %v", sf, tc.short)
			}
		})
	}
}

func TestHeldRunsTakeTheirShareFirst(t *testing.T) {
	ssd := []provider.Quota{{Name: "compute.ssdDisks.size", Zone: "ru-central1-d", Limit: 1600, Used: 0, Unit: "GiB"}}
	big := pipelinespec.Run{
		Provider: pipelinespec.Provider{Kind: pipelinespec.ProviderYandex, Settings: []byte(`{"zone":"ru-central1-d"}`)},
		Network:  pipelinespec.Network{AllowPublicIPs: true},
		Machines: []pipelinespec.Machine{{Disks: []pipelinespec.Disk{{GB: 400, Type: "network-ssd"}}}, {Disks: []pipelinespec.Disk{{GB: 400, Type: "network-ssd"}}}},
	}
	if sf, _ := quotaShortfall(ssd, nil, big); sf != nil {
		t.Fatalf("880 GiB fits into 1600: %v", sf)
	}
	sf, never := quotaShortfall(ssd, []pipelinespec.Run{big}, big)
	if sf == nil || never || sf.free != 720 || sf.need != 880 {
		t.Fatalf("a started run not yet in the cloud's usage must hold its 880 GiB: %+v never=%v", sf, never)
	}
}

func TestNetworkQuotaNeeds(t *testing.T) {
	yandex := func(settings string, public bool, zones ...string) pipelinespec.Run {
		r := pipelinespec.Run{
			Provider: pipelinespec.Provider{Kind: pipelinespec.ProviderYandex, Settings: []byte(settings)},
			Network:  pipelinespec.Network{AllowPublicIPs: public},
		}
		for _, z := range zones {
			r.Machines = append(r.Machines, pipelinespec.Machine{Location: z})
		}
		return r
	}
	public := yandex(`{"zone":"ru-central1-d"}`, true, "ru-central1-a", "ru-central1-b", "ru-central1-a")
	sharedNAT := yandex(`{"zone":"ru-central1-d","public_ips":false,"network":{"kind":"create"}}`, false, "ru-central1-d")
	ownNAT := yandex(`{"zone":"ru-central1-d"}`, false, "ru-central1-d")
	aws := pipelinespec.Run{Provider: pipelinespec.Provider{Kind: pipelinespec.ProviderAWS}, Network: pipelinespec.Network{AllowPublicIPs: true}, Machines: []pipelinespec.Machine{{}}}
	for _, tc := range []struct {
		name  string
		run   pipelinespec.Run
		quota string
		need  float64
	}{
		// The profile network is shared: a run takes no network.
		{"yandex network", public, "vpc.networks.count", 0},
		{"yandex group", public, "vpc.securityGroups.count", 1},
		{"yandex subnet per zone", public, "vpc.subnets.count", 2},
		{"public needs no gateway", public, "vpc.gateways.count", 0},
		{"private on the shared NAT", sharedNAT, "vpc.gateways.count", 0},
		{"private on the shared routes", sharedNAT, "vpc.routeTables.count", 0},
		{"private without a profile NAT brings its own", ownNAT, "vpc.gateways.count", 1},
		{"aws vpc", aws, "vpc.vpcs", 1},
		{"aws group", aws, "vpc.security_groups", 1},
		{"aws gateway", aws, "vpc.internet_gateways", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			total, _ := runDemand(tc.run)
			need, ok := total.need(provider.Quota{Name: tc.quota}, tc.run)
			if !ok || need != tc.need {
				t.Fatalf("need %v (known %v), want %v", need, ok, tc.need)
			}
		})
	}
	// Ten networks used out of ten no longer hold a Yandex run.
	if sf, _ := quotaShortfall([]provider.Quota{{Name: "vpc.networks.count", Limit: 10, Used: 10}}, nil, public); sf != nil {
		t.Fatalf("a full network quota held a run on the profile network: %+v", sf)
	}
}
