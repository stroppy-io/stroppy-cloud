package share

import (
	"context"

	"github.com/google/uuid"
)

// BackfillTopology copies the topology shape into the runs of share
// snapshots captured before it, from the runs themselves (run their own
// backfill first). The snapshot stays as captured otherwise.
func (s *Service) BackfillTopology(ctx context.Context) (int, error) {
	shares, err := s.repo.All(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, sh := range shares {
		missing := map[uuid.UUID]bool{}
		sh.Snapshot.eachRun(func(r *SharedRun) {
			if r.Summary.Topology.IsZero() {
				missing[r.ID] = true
			}
		})
		if len(missing) == 0 {
			continue
		}
		ids := make([]uuid.UUID, 0, len(missing))
		for id := range missing {
			ids = append(ids, id)
		}
		runs, err := s.runs.ByIDs(ctx, ids)
		if err != nil {
			return n, err
		}
		changed := false
		for _, r := range runs {
			if r.Summary.Topology.IsZero() {
				continue
			}
			sh.Snapshot.eachRun(func(x *SharedRun) {
				if x.ID == r.ID && x.Summary.Topology.IsZero() {
					x.Summary.Topology, changed = r.Summary.Topology, true
				}
			})
		}
		if !changed {
			continue
		}
		if err := s.repo.SetSnapshot(ctx, sh.ID, sh.Snapshot, sh.CapturedAt, sh.Title); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// eachRun visits every run projection of the snapshot.
func (snap *Snapshot) eachRun(fn func(*SharedRun)) {
	if snap.Run != nil {
		fn(snap.Run)
	}
	if snap.SuiteRun != nil {
		for i := range snap.SuiteRun.Cells {
			fn(&snap.SuiteRun.Cells[i])
		}
	}
	for id, r := range snap.Runs {
		fn(&r)
		snap.Runs[id] = r
	}
}
