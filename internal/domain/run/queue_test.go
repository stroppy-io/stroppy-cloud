package run

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/provider"
	pipelinespec "github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

type queueRepo struct {
	Repository
	cells    map[uuid.UUID]int
	starting []Starting
	network  []NetworkRun
}

func (r queueRepo) NetworkRuns(context.Context, uuid.UUID, uuid.UUID, time.Time) ([]NetworkRun, error) {
	return r.network, nil
}

func (r queueRepo) LiveCells(context.Context, uuid.UUID) (map[uuid.UUID]int, error) {
	out := map[uuid.UUID]int{}
	for k, v := range r.cells {
		out[k] = v
	}
	return out, nil
}

func (r queueRepo) Starting(context.Context, uuid.UUID) ([]Starting, error) { return r.starting, nil }

type queuePipelines struct {
	ready  bool
	status string
}

func (p queuePipelines) PipelinesReady(context.Context, string) (bool, string) {
	return p.ready, p.status
}

type queueAccess struct {
	Access
	suspended bool
}

func (a queueAccess) Suspended(context.Context, uuid.UUID) (bool, error) { return a.suspended, nil }

type queueSuites map[uuid.UUID]int

func (s queueSuites) SuiteConcurrency(_ context.Context, id uuid.UUID) (int, error) {
	return s[id], nil
}

func mustJSON(v any) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return raw
}

// poolSpec is a one-machine Yandex run on a profile with its own pool.
func poolSpec(pool string) json.RawMessage {
	return mustJSON(pipelinespec.Run{
		Provider: pipelinespec.Provider{Kind: pipelinespec.ProviderYandex, Settings: json.RawMessage(`{"zone":"ru-central1-d","network":{"kind":"existing","network_id":"enp2v5nl4h1mabcdefgh","pool_cidr":"` + pool + `"}}`)},
		Network:  pipelinespec.Network{AllowPublicIPs: true},
		Machines: []pipelinespec.Machine{{Name: "runner-1"}},
	})
}

func TestQueueAllocatesSubnets(t *testing.T) {
	profile := uuid.New()
	queued := func(raw json.RawMessage) Run {
		r := Run{ID: uuid.New(), Status: StatusPending, RunSpec: raw}
		r.Snapshot.ProviderProfile.ID = profile
		return r
	}
	resume := mustJSON(pipelinespec.Run{
		Provider: pipelinespec.Provider{Kind: pipelinespec.ProviderYandex, Settings: json.RawMessage(`{"zone":"ru-central1-d"}`)},
		Machines: []pipelinespec.Machine{{Name: "runner-1"}},
		Resume:   &pipelinespec.Resume{FromRunID: uuid.NewString()},
	})
	// An earlier run still holds the first block; the cursor is after it.
	held := mustJSON(pipelinespec.Run{Network: pipelinespec.Network{Subnets: []pipelinespec.Subnet{{Zone: "ru-central1-d", CIDR: "10.9.0.0/28"}}}})
	svc := &Service{access: queueAccess{}, repo: queueRepo{network: []NetworkRun{{Holds: true, Spec: held}}}}
	runs := []Run{queued(poolSpec("10.9.0.0/24")), queued(resume), queued(poolSpec("10.9.0.0/24"))}
	quotas := map[uuid.UUID]quotaRead{profile: {profile: provider.Profile{Kind: provider.KindYandex, Status: provider.StatusReady, ConfigRevision: provider.ConfigRevision}}}
	d, err := svc.decide(t.Context(), uuid.New(), runs, quotas)
	if err != nil {
		t.Fatal(err)
	}
	for i := range runs {
		if !d.admit[i] {
			t.Fatalf("run %d waits: %+v", i, d.waits[i])
		}
	}
	if d.specs[1] != nil {
		t.Fatal("a resumed run keeps the stand's subnets and allocates nothing")
	}
	var got []string
	for _, i := range []int{0, 2} {
		var spec pipelinespec.Run
		if err := json.Unmarshal(d.specs[i], &spec); err != nil {
			t.Fatal(err)
		}
		if len(spec.Network.Subnets) != 1 || spec.Network.Subnets[0].Zone != "ru-central1-d" || len(spec.Machines) != 1 {
			t.Fatalf("run %d spec: %+v", i, spec)
		}
		got = append(got, spec.Network.Subnets[0].CIDR)
	}
	if got[0] != "10.9.0.16/28" || got[1] != "10.9.0.32/28" {
		t.Fatalf("allocated %v, want the blocks after the held one in turn", got)
	}
}

func TestQueueDecides(t *testing.T) {
	profile := uuid.New()
	zone := []byte(`{"zone":"ru-central1-d"}`)
	spec := func(disks ...int) json.RawMessage {
		r := pipelinespec.Run{Provider: pipelinespec.Provider{Kind: pipelinespec.ProviderYandex, Settings: zone}, Network: pipelinespec.Network{AllowPublicIPs: true}}
		for _, gb := range disks {
			r.Machines = append(r.Machines, pipelinespec.Machine{Disks: []pipelinespec.Disk{{GB: gb, Type: "network-ssd"}}})
		}
		raw, _ := json.Marshal(r)
		return raw
	}
	queued := func(s json.RawMessage, suite *uuid.UUID) Run {
		r := Run{ID: uuid.New(), Status: StatusPending, RunSpec: s, SuiteRunID: suite}
		r.Snapshot.ProviderProfile.ID = profile
		return r
	}
	ssd := func(used float64) map[uuid.UUID]quotaRead {
		return map[uuid.UUID]quotaRead{profile: {profile: provider.Profile{Status: provider.StatusReady}, quotas: []provider.Quota{{Name: "compute.ssdDisks.size", Zone: "ru-central1-d", Limit: 1600, Used: used, Unit: "GiB"}}}}
	}
	sr := uuid.New()
	for _, tc := range []struct {
		name      string
		svc       *Service
		runs      []Run
		quotas    map[uuid.UUID]quotaRead
		admit     []bool
		waitingOn []WaitReason
	}{
		{
			name:      "a big run waits for SSD, the small one behind it goes first",
			svc:       &Service{access: queueAccess{}, repo: queueRepo{}},
			runs:      []Run{queued(spec(400, 400, 400), nil), queued(spec(400), nil)},
			quotas:    ssd(500),
			admit:     []bool{false, true},
			waitingOn: []WaitReason{WaitQuota, ""},
		},
		{
			name: "runs share the profile network: no network quota, a subnet each",
			svc:  &Service{access: queueAccess{}, repo: queueRepo{}},
			runs: []Run{queued(spec(40), nil), queued(spec(40), nil), queued(spec(40), nil)},
			quotas: map[uuid.UUID]quotaRead{profile: {profile: provider.Profile{Status: provider.StatusReady}, quotas: []provider.Quota{
				{Name: "vpc.networks.count", Limit: 10, Used: 10, Unit: "networks"},
				{Name: "vpc.subnets.count", Limit: 32, Used: 30, Unit: "subnets"},
				{Name: "vpc.securityGroups.count", Limit: 32, Used: 5, Unit: "security-groups"},
			}}},
			admit:     []bool{true, true, false},
			waitingOn: []WaitReason{"", "", WaitQuota},
		},
		{
			name:      "a Yandex profile without its network holds its runs",
			svc:       &Service{access: queueAccess{}, repo: queueRepo{}},
			runs:      []Run{queued(spec(40), nil)},
			quotas:    map[uuid.UUID]quotaRead{profile: {profile: provider.Profile{Kind: provider.KindYandex, Status: provider.StatusReady}}},
			admit:     []bool{false},
			waitingOn: []WaitReason{WaitProvider},
		},
		{
			name: "a full address pool holds the run",
			svc: &Service{access: queueAccess{}, repo: queueRepo{network: []NetworkRun{{Holds: true, Spec: mustJSON(pipelinespec.Run{Network: pipelinespec.Network{Subnets: []pipelinespec.Subnet{
				{Zone: "ru-central1-d", CIDR: "10.0.0.0/25"}, {Zone: "ru-central1-a", CIDR: "10.0.0.128/25"},
			}}})}}}},
			runs:      []Run{queued(poolSpec("10.0.0.0/24"), nil)},
			quotas:    ssd(0),
			admit:     []bool{false},
			waitingOn: []WaitReason{WaitSubnets},
		},
		{
			name:      "an empty queue under the lock decides nothing",
			svc:       &Service{access: queueAccess{}, repo: queueRepo{}, pipelines: queuePipelines{ready: true}},
			runs:      []Run{},
			quotas:    ssd(0),
			admit:     []bool{},
			waitingOn: []WaitReason{},
		},
		{
			name:      "no cap on live runs: room in the quota is the only gate",
			svc:       &Service{access: queueAccess{}, repo: queueRepo{}},
			runs:      []Run{queued(spec(40), nil), queued(spec(40), nil)},
			quotas:    ssd(0),
			admit:     []bool{true, true},
			waitingOn: []WaitReason{"", ""},
		},
		{
			name:      "a started run not in the cloud's usage yet holds its SSD",
			svc:       &Service{access: queueAccess{}, repo: queueRepo{starting: []Starting{{ProfileID: profile, Spec: spec(400, 400, 400)}}}},
			runs:      []Run{queued(spec(400), nil)},
			quotas:    ssd(0),
			admit:     []bool{false},
			waitingOn: []WaitReason{WaitQuota},
		},
		{
			name:      "suite concurrency",
			svc:       &Service{access: queueAccess{}, repo: queueRepo{cells: map[uuid.UUID]int{sr: 1}}, suites: queueSuites{sr: 2}},
			runs:      []Run{queued(spec(40), &sr), queued(spec(40), &sr), queued(spec(40), nil)},
			quotas:    ssd(0),
			admit:     []bool{true, false, true},
			waitingOn: []WaitReason{"", WaitSuiteSlot, ""},
		},
		{
			name:      "a suspended tenant starts nothing",
			svc:       &Service{access: queueAccess{suspended: true}, repo: queueRepo{}},
			runs:      []Run{queued(spec(40), nil)},
			quotas:    ssd(0),
			admit:     []bool{false},
			waitingOn: []WaitReason{WaitSuspended},
		},
		{
			name:      "pipelines not pushed hold everything",
			svc:       &Service{access: queueAccess{}, repo: queueRepo{}, pipelines: queuePipelines{status: "pending"}},
			runs:      []Run{queued(spec(40), nil), queued(spec(40), nil)},
			quotas:    ssd(0),
			admit:     []bool{false, false},
			waitingOn: []WaitReason{WaitPipelines, WaitPipelines},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, err := tc.svc.decide(t.Context(), uuid.New(), tc.runs, tc.quotas)
			if err != nil {
				t.Fatal(err)
			}
			for i := range tc.runs {
				if d.admit[i] != tc.admit[i] || d.waits[i].WaitingFor != tc.waitingOn[i] {
					t.Fatalf("run %d: admit %v waits %+v, want admit %v waiting for %q", i, d.admit[i], d.waits[i], tc.admit[i], tc.waitingOn[i])
				}
				if !d.admit[i] && d.waits[i].Position != i+1 {
					t.Fatalf("run %d: position %d", i, d.waits[i].Position)
				}
			}
		})
	}
}
