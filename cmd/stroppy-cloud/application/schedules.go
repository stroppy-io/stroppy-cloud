package application

import (
	"context"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/library"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/schedule"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/suite"
)

// scheduleLauncher adapts the run and suite services to the schedule's
// Launcher port.
type scheduleLauncher struct {
	runs   *run.Service
	suites *suite.Service
	lib    *library.Service
}

func (l scheduleLauncher) LaunchTest(ctx context.Context, actor auth.Actor, tenantID, testID uuid.UUID, o run.Overrides) (id uuid.UUID, name, status string, err error) {
	r, err := l.runs.Launch(ctx, actor, tenantID, testID, o, "")
	if err != nil {
		return uuid.Nil, "", "", err
	}
	return r.ID, r.Name, string(r.Status), nil
}

func (l scheduleLauncher) LaunchSuite(ctx context.Context, actor auth.Actor, tenantID, suiteID uuid.UUID, o run.Overrides) (id uuid.UUID, name, status string, err error) {
	in := suite.Launch{Name: o.Name, Keep: o.Keep, RatingTenant: o.RatingTenant, RatingGlobal: o.RatingGlobal, Labels: o.Labels, Trigger: run.TriggerSchedule, ScheduleID: o.ScheduleID}
	r, err := l.suites.LaunchSuite(ctx, actor, tenantID, suiteID, in, "")
	if err != nil {
		return uuid.Nil, "", "", err
	}
	return r.ID, r.Name, string(r.Status), nil
}

func (l scheduleLauncher) TargetName(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, kind schedule.TargetKind, id uuid.UUID) (string, error) {
	switch kind {
	case schedule.TargetTest:
		t, _, _, err := l.lib.GetTest(ctx, actor, tenantID, id)
		if err != nil {
			return "", err
		}
		return t.Name, nil
	case schedule.TargetSuite:
		s, err := l.suites.Get(ctx, actor, tenantID, id)
		if err != nil {
			return "", err
		}
		return s.Suite.Name, nil
	default:
		return "", errs.Invalid("target.kind must be test or suite")
	}
}

// testUsers reports suites and schedules referencing a test (library
// delete guard).
type testUsers struct {
	suites    *suite.Service
	schedules interface {
		SchedulesOf(ctx context.Context, kind string, id uuid.UUID) ([]run.Ref, error)
	}
}

func (u testUsers) UsingTest(ctx context.Context, testID uuid.UUID) ([]library.Usage, error) {
	out, err := u.suites.UsingTest(ctx, testID)
	if err != nil {
		return nil, err
	}
	refs, err := u.schedules.SchedulesOf(ctx, "test", testID)
	if err != nil {
		return nil, err
	}
	for _, r := range refs {
		out = append(out, library.Usage{Kind: "schedule", ID: r.ID, Name: r.Name})
	}
	return out, nil
}
