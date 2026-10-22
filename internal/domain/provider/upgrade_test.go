package provider

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

type upgradeRepo struct {
	Repository
	stale    []Profile
	busy     map[uuid.UUID]bool
	begun    []uuid.UUID
	finished map[uuid.UUID]Status
	reasons  []string
}

func (r *upgradeRepo) Stale(_ context.Context, kind Kind, revision int) ([]Profile, error) {
	if kind != KindYandex || revision != ConfigRevision {
		return nil, nil
	}
	return r.stale, nil
}

func (r *upgradeRepo) BeginUpgrade(_ context.Context, id uuid.UUID, _ string, _ int) (bool, error) {
	if r.busy[id] {
		return false, nil
	}
	r.begun = append(r.begun, id)
	return true, nil
}

func (r *upgradeRepo) FinishOperation(_ context.Context, id uuid.UUID, _ string, status Status, _ string) error {
	r.finished[id] = status
	return nil
}

func (r *upgradeRepo) RetryUpgrade(_ context.Context, id uuid.UUID, _, reason string) error {
	r.finished[id] = StatusReady
	r.reasons = append(r.reasons, reason)
	return nil
}

func (r *upgradeRepo) SetQuotas(context.Context, uuid.UUID, QuotasResult) error { return nil }

type upgradePipelines struct {
	Pipelines
	actions []string
	fail    string
}

func (p *upgradePipelines) Configure(_ context.Context, _ string, params ConfigureParams) (VerifyResult, error) {
	p.actions = append(p.actions, params.Action)
	if p.fail != "" {
		return VerifyResult{OK: false, Error: p.fail}, nil
	}
	return VerifyResult{OK: true}, nil
}

func (p *upgradePipelines) Quotas(context.Context, string, QuotasParams) (QuotasResult, error) {
	return QuotasResult{}, nil
}

type upgradeAccess struct{ Access }

func (upgradeAccess) NamespaceOf(context.Context, uuid.UUID) (string, error) { return "t-acme", nil }

func TestProfileConfigured(t *testing.T) {
	for _, tc := range []struct {
		p    Profile
		want bool
	}{
		{Profile{Kind: KindYandex}, false},
		{Profile{Kind: KindYandex, ConfigRevision: ConfigRevision}, true},
		{Profile{Kind: KindAWS}, true},
	} {
		if got := tc.p.Configured(); got != tc.want {
			t.Fatalf("%+v configured %v, want %v", tc.p, got, tc.want)
		}
	}
}

func TestUpgradeStaleConfiguresOldYandexProfiles(t *testing.T) {
	idle, busy := uuid.New(), uuid.New()
	repo := &upgradeRepo{
		stale:    []Profile{{ID: idle, Kind: KindYandex, Status: StatusReady}, {ID: busy, Kind: KindYandex, Status: StatusReady}},
		busy:     map[uuid.UUID]bool{busy: true},
		finished: map[uuid.UUID]Status{},
	}
	pipes := &upgradePipelines{}
	svc := NewService(repo, nil, pipes, nil, upgradeAccess{}, func(ctx context.Context, _ string) context.Context { return ctx }, nil, func(fn func(context.Context)) { fn(context.Background()) })
	if err := svc.UpgradeStale(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(repo.begun) != 1 || repo.begun[0] != idle {
		t.Fatalf("upgraded %v, want only the idle profile", repo.begun)
	}
	if len(pipes.actions) != 1 || pipes.actions[0] != "ensure" {
		t.Fatalf("pipeline actions %v", pipes.actions)
	}
	if repo.finished[idle] != StatusReady {
		t.Fatalf("idle profile ended %q", repo.finished[idle])
	}
	if _, touched := repo.finished[busy]; touched {
		t.Fatal("a profile the repository did not hand over was configured")
	}
}

// Right after a deploy the namespace runs the previous provider-config:
// the upgrade waits until this server's pipelines are pushed.
func TestUpgradeStaleWaitsForPipelines(t *testing.T) {
	repo := &upgradeRepo{
		stale:    []Profile{{ID: uuid.New(), Kind: KindYandex, Status: StatusReady}},
		busy:     map[uuid.UUID]bool{},
		finished: map[uuid.UUID]Status{},
	}
	pipes := &upgradePipelines{}
	svc := NewService(repo, nil, pipes, nil, upgradeAccess{}, func(ctx context.Context, _ string) context.Context { return ctx }, nil, func(fn func(context.Context)) { fn(context.Background()) })
	ready := false
	svc.WithPipelineReadiness(func(context.Context, string) bool { return ready })
	if err := svc.UpgradeStale(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(repo.begun) != 0 || len(pipes.actions) != 0 {
		t.Fatalf("upgraded before the pipelines: %v %v", repo.begun, pipes.actions)
	}
	ready = true
	if err := svc.UpgradeStale(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(repo.begun) != 1 {
		t.Fatalf("upgraded %v after the pipelines", repo.begun)
	}
}

// A failed upgrade (the cloud's network quota still held by old runs)
// leaves the profile ready on its old configuration, to be tried again.
func TestFailedUpgradeKeepsTheProfileReady(t *testing.T) {
	id := uuid.New()
	repo := &upgradeRepo{stale: []Profile{{ID: id, Kind: KindYandex, Status: StatusReady}}, busy: map[uuid.UUID]bool{}, finished: map[uuid.UUID]Status{}}
	pipes := &upgradePipelines{fail: "Quota limit vpc.networks.count exceeded"}
	svc := NewService(repo, nil, pipes, nil, upgradeAccess{}, func(ctx context.Context, _ string) context.Context { return ctx }, nil, func(fn func(context.Context)) { fn(context.Background()) })
	if err := svc.UpgradeStale(t.Context()); err != nil {
		t.Fatal(err)
	}
	if repo.finished[id] != StatusReady || len(repo.reasons) != 1 {
		t.Fatalf("profile ended %q with reasons %v", repo.finished[id], repo.reasons)
	}
}

// After a restart the pending operation resumes as an upgrade: its failure
// keeps the profile ready.
func TestRecoveredUpgradeKeepsTheProfileReady(t *testing.T) {
	id := uuid.New()
	repo := &upgradeRepo{busy: map[uuid.UUID]bool{}, finished: map[uuid.UUID]Status{}}
	pipes := &upgradePipelines{fail: "Quota limit vpc.networks.count exceeded"}
	svc := NewService(repo, nil, pipes, nil, upgradeAccess{}, func(ctx context.Context, _ string) context.Context { return ctx }, nil, func(fn func(context.Context)) { fn(context.Background()) })
	svc.startVerify("t-acme", Profile{ID: id, Kind: KindYandex, Status: StatusVerifying, StatusReason: UpgradeReason, VerifyRunID: "r"})
	if repo.finished[id] != StatusReady {
		t.Fatalf("recovered upgrade ended %q", repo.finished[id])
	}
	svc.startVerify("t-acme", Profile{ID: id, Kind: KindYandex, Status: StatusVerifying, VerifyRunID: "v"})
	if repo.finished[id] != StatusFailed {
		t.Fatalf("a verification ended %q", repo.finished[id])
	}
}
