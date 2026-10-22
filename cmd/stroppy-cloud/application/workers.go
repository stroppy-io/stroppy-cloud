package application

import (
	"context"
	"time"

	"github.com/gopherex/xlog"
)

/*
WORKERS: the background loops of the process, all under the shutdown
manager. Webhook dispatch and the quota sweep are idempotent and lease
their rows, so several replicas may run them.
*/

const (
	webhookTick   = 5 * time.Second
	quotaSweep    = 15 * time.Minute
	projectorTick = 3 * time.Second
	// queueTick re-checks queued runs; launches, finished runs and pushed
	// pipelines nudge the queue sooner.
	queueTick    = 15 * time.Second
	scheduleTick = 30 * time.Second
	// qpsBackfillConcurrency bounds the metric store queries of the
	// startup series backfill.
	qpsBackfillConcurrency = 4
)

func (a *Application) startWorkers() {
	a.shutdown.Go(func(ctx context.Context) {
		a.services.Dispatcher.Run(ctx, webhookTick)
	})
	a.shutdown.Go(func(ctx context.Context) {
		a.services.Projector.Run(ctx, projectorTick)
	})
	a.shutdown.Go(func(ctx context.Context) {
		a.services.SuiteProj.Run(ctx, projectorTick)
	})
	a.shutdown.Go(func(ctx context.Context) {
		a.services.Runs.RunQueue(ctx, queueTick)
	})
	// One pass at startup: finished runs without a stored throughput
	// series (older than the series, or finished while the metric store
	// was unreachable) get one. Idempotent: a run is asked once.
	a.shutdown.Go(func(ctx context.Context) {
		stored, empty, skipped := a.services.Projector.BackfillQPSSeries(ctx, qpsBackfillConcurrency)
		if stored+empty+skipped > 0 {
			a.log.Ctx().Info(ctx, "qps series backfill", xlog.Int("stored", stored), xlog.Int("empty", empty), xlog.Int("skipped", skipped))
		}
	})
	// One pass at startup: definitions, runs and share snapshots stored
	// before the topology shape get one (runs before shares: shares copy
	// the run's). Idempotent.
	a.shutdown.Go(func(ctx context.Context) {
		dbs, err := a.services.Library.BackfillTopology(ctx)
		if err != nil {
			a.log.Ctx().Warn(ctx, "topology backfill: databases", xlog.ErrorCause(err))
		}
		runs, err := a.services.Runs.BackfillTopology(ctx)
		if err != nil {
			a.log.Ctx().Warn(ctx, "topology backfill: runs", xlog.ErrorCause(err))
		}
		shares, err := a.services.Shares.BackfillTopology(ctx)
		if err != nil {
			a.log.Ctx().Warn(ctx, "topology backfill: shares", xlog.ErrorCause(err))
		}
		if dbs+runs+shares > 0 {
			a.log.Ctx().Info(ctx, "topology backfill", xlog.Int("databases", dbs), xlog.Int("runs", runs), xlog.Int("shares", shares))
		}
	})
	a.shutdown.Go(func(ctx context.Context) {
		a.services.Schedules.Run(ctx, scheduleTick)
	})
	if a.cfg.Infra.Pipelines.Dir != "" {
		a.shutdown.Go(func(ctx context.Context) {
			a.services.Pipelines.Run(ctx, a.cfg.Infra.Pipelines.Interval)
		})
	}
	a.shutdown.Go(func(ctx context.Context) {
		recoverProviders := func(ctx context.Context) {
			if err := a.services.Providers.RecoverPending(ctx); err != nil {
				a.log.Ctx().Warn(ctx, "provider recovery failed", xlog.ErrorCause(err))
			}
			// Yandex profiles from before the profile network get one; their
			// queued runs wait for it (run queue: provider not configured).
			if err := a.services.Providers.UpgradeStale(ctx); err != nil {
				a.log.Ctx().Warn(ctx, "provider upgrade failed", xlog.ErrorCause(err))
			}
		}
		recoverProviders(ctx)
		every(ctx, 15*time.Second, recoverProviders)
	})
	a.shutdown.Go(func(ctx context.Context) {
		every(ctx, quotaSweep, func(ctx context.Context) {
			if err := a.services.Providers.RefreshAllReady(ctx); err != nil {
				a.log.Ctx().Warn(ctx, "quota sweep failed", xlog.ErrorCause(err))
			}
		})
	})
}

// every runs fn on a ticker until ctx ends (first run after one interval).
func every(ctx context.Context, interval time.Duration, fn func(ctx context.Context)) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			fn(ctx)
		}
	}
}
