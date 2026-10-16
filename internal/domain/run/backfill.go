package run

import (
	"context"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
)

// BackfillTopology stores the topology shape of runs launched before it:
// derived from the snapshot's database as at launch, else counted from the
// snapshot's machines (a definition the catalog no longer derives).
func (s *Service) BackfillTopology(ctx context.Context) (int, error) {
	runs, err := s.repo.WithoutTopology(ctx, 1000)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, r := range runs {
		shape := machinesShape(r.Snapshot.Machines)
		if s.library != nil {
			if _, derived, err := s.library.DeriveDatabase(ctx, r.Snapshot.Database); err == nil {
				shape = derived.Plan.Shape()
			}
		}
		if err := s.repo.SetTopology(ctx, r.ID, shape); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// machinesShape counts the machines by role, in their order.
func machinesShape(ms []MachineSnapshot) topology.Shape {
	out := topology.Shape{Nodes: []topology.ShapeNode{}}
	index := map[string]int{}
	for _, m := range ms {
		i, ok := index[m.Role]
		if !ok {
			i = len(out.Nodes)
			index[m.Role] = i
			out.Nodes = append(out.Nodes, topology.ShapeNode{Role: m.Role})
		}
		out.Nodes[i].Count++
	}
	return out
}
