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
	denylistPurge = time.Hour
	projectorTick = 3 * time.Second
	scheduleTick  = 30 * time.Second
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
	a.shutdown.Go(func(ctx context.Context) {
		every(ctx, denylistPurge, func(ctx context.Context) {
			if err := a.services.IAM.Purge(ctx, time.Now().Add(-7*24*time.Hour)); err != nil {
				a.log.Ctx().Warn(ctx, "iam purge failed", xlog.ErrorCause(err))
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
