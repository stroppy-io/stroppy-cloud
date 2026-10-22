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

func (r *upgradeRepo) SetQuotas(context.Context, uuid.UUID, QuotasResult) error { return nil }

type upgradePipelines struct {
	Pipelines
	actions []string
}

func (p *upgradePipelines) Configure(_ context.Context, _ string, params ConfigureParams) (VerifyResult, error) {
	p.actions = append(p.actions, params.Action)
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
		t.Fatal("a profile with a started run or a kept stand was configured")
	}
}
