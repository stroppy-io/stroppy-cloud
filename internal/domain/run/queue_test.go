package run

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/provider"
	pipelinespec "github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

type queueRepo struct {
	Repository
	cells    map[uuid.UUID]int
	starting []Starting
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
			name: "each run takes a network: the second waits when one is left",
			svc:  &Service{access: queueAccess{}, repo: queueRepo{}},
			runs: []Run{queued(spec(40), nil), queued(spec(40), nil)},
			quotas: map[uuid.UUID]quotaRead{profile: {profile: provider.Profile{Status: provider.StatusReady}, quotas: []provider.Quota{
				{Name: "vpc.networks.count", Limit: 6, Used: 5, Unit: "count"},
				{Name: "vpc.subnets.count", Limit: 32, Used: 5, Unit: "count"},
			}}},
			admit:     []bool{true, false},
			waitingOn: []WaitReason{"", WaitQuota},
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
