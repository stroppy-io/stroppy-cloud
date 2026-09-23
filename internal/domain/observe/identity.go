package observe

import (
	"encoding/json"
	"sort"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// Scope is where a run's telemetry is: its Graphene namespace and the id
// Graphene knows the run by (a suite cell is "<suite run>-<cell>").
type Scope struct {
	Namespace string
	Run       string
}

// ScopeOf is the run's telemetry scope.
func ScopeOf(r run.Run) Scope {
	return Scope{Namespace: r.GrapheneNamespace, Run: r.GrapheneID()}
}

// identities translates between the run's plan (roles, machines,
// containers) and the telemetry's agents and entities, with the pipeline's
// own naming (spec.AgentName, spec.ContainerEntity).
type identities struct {
	// agent → machine, machine → role, entity → container.
	machineOf   map[string]string
	roleOf      map[string]string
	containerOf map[string]string
	// container → machine.
	hostOf map[string]string
	runID  string
}

func identitiesOf(r run.Run) identities {
	id := identities{machineOf: map[string]string{}, roleOf: map[string]string{}, containerOf: map[string]string{}, hostOf: map[string]string{}}
	var rs spec.Run
	if json.Unmarshal(r.RunSpec, &rs) != nil || rs.RunID == "" {
		return id
	}
	id.runID = rs.RunID
	for _, m := range rs.Machines {
		id.machineOf[spec.AgentName(rs.RunID, m.Name)] = m.Name
		id.roleOf[m.Name] = m.Role
	}
	for _, c := range rs.Containers {
		id.containerOf[spec.ContainerEntity(rs.RunID, c.Name)] = c.Name
		id.hostOf[c.Name] = c.Machine
	}
	return id
}

// agents are the agents of the machines named or holding the roles named.
func (id identities) agents(roles, machines []string) []string {
	if len(roles) == 0 && len(machines) == 0 {
		return nil
	}
	want := map[string]bool{}
	for _, m := range machines {
		want[m] = true
	}
	for _, r := range roles {
		for m, role := range id.roleOf {
			if role == r {
				want[m] = true
			}
		}
	}
	out := []string{}
	for agent, m := range id.machineOf {
		if want[m] {
			out = append(out, agent)
		}
	}
	sort.Strings(out)
	if len(out) == 0 {
		// Nothing matches: a filter that selects nothing, not everything.
		out = []string{"-"}
	}
	return out
}

// entities are the entities of the containers named.
func (id identities) entities(containers []string) []string {
	if len(containers) == 0 {
		return nil
	}
	out := make([]string, 0, len(containers))
	for _, c := range containers {
		out = append(out, spec.ContainerEntity(id.runID, c))
	}
	sort.Strings(out)
	return out
}

// enrich fills a line's machine, role and container from its agent and entity.
func (id identities) enrich(l *LogLine) {
	if c, ok := id.containerOf[l.Fields[spec.AttrEntity]]; ok && l.Container == "" {
		l.Container = c
		if l.Machine == "" {
			l.Machine = id.hostOf[c]
		}
	}
	if m, ok := id.machineOf[l.Fields[spec.AttrAgent]]; ok && l.Machine == "" {
		l.Machine = m
	}
	if l.Role == "" {
		l.Role = id.roleOf[l.Machine]
	}
}

// series fills a series' machine and role from its agent.
func (id identities) series(s *Series) {
	if m, ok := id.machineOf[s.Machine]; ok {
		s.Machine = m
	}
	if s.Role == "" {
		s.Role = id.roleOf[s.Machine]
	}
}

// facets turns agent/entity facets into machine, role and container ones.
func (id identities) facets(raw []run.Facet) []run.Facet {
	out := []run.Facet{}
	var machines, roles, containers map[string]int
	for _, f := range raw {
		switch f.Field {
		case spec.AttrAgent:
			machines, roles = map[string]int{}, map[string]int{}
			for _, v := range f.Values {
				if m, ok := id.machineOf[v.Value]; ok {
					machines[m] += v.Count
					roles[id.roleOf[m]] += v.Count
				}
			}
		case spec.AttrEntity:
			containers = map[string]int{}
			for _, v := range f.Values {
				if c, ok := id.containerOf[v.Value]; ok {
					containers[c] += v.Count
				}
			}
		default:
			out = append(out, f)
		}
	}
	add := func(field string, counts map[string]int) {
		if len(counts) == 0 {
			return
		}
		f := run.Facet{Field: field, Values: []run.FacetValue{}}
		for v, n := range counts {
			if v != "" {
				f.Values = append(f.Values, run.FacetValue{Value: v, Count: n})
			}
		}
		sort.Slice(f.Values, func(i, j int) bool { return f.Values[i].Count > f.Values[j].Count })
		out = append(out, f)
	}
	add("machine", machines)
	add("role", roles)
	add("container", containers)
	sort.Slice(out, func(i, j int) bool { return out[i].Field < out[j].Field })
	return out
}
