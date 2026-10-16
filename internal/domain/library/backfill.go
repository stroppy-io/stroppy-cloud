package library

import (
	"context"
	"fmt"
)

// backfillLimit bounds one backfill pass.
const backfillLimit = 1000

// BackfillTopology stores the topology shape of definitions saved before
// it: derived from their params as on save. A definition that no longer
// derives (a retired version) gets an empty shape and is not asked again.
func (s *Service) BackfillTopology(ctx context.Context) (int, error) {
	ids, err := s.repo.DatabasesWithoutTopology(ctx, backfillLimit)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, id := range ids {
		d, err := s.repo.DatabaseByID(ctx, id)
		if err != nil {
			return n, fmt.Errorf("database %s: %w", id, err)
		}
		spec, _, err := s.DeriveDatabase(ctx, d.Spec)
		if err != nil {
			spec = d.Spec
		}
		if err := s.repo.SetDatabaseTopology(ctx, id, spec.Topology); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}
