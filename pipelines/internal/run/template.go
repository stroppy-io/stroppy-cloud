// Package run is the orchestration of stroppy-run: it turns a RunSpec into
// cloud machines, agents, containers, a workload and a result, phase by
// phase, emitting milestones the server projects into the run overview.
//
// The workflow code here only declares resources and dispatches activities;
// bodies live in internal/activities, cloud shapes in internal/provision.
package run

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/provision"
)

// Placeholders the server may leave in a RunSpec where a machine address is
// needed but unknown at compile time (private IPs exist only after
// provisioning):
//
//	${ip:<machine>}        private IP of the named machine
//	${ip:role:<role>}      private IP of the first machine of the role
//	${ips:role:<role>}     comma-separated private IPs of every machine of the role
//	${public_ip:<machine>} public IP of the named machine (empty when none)
//
// They are expanded in container env/cmd/files and in the workload env.
var placeholder = regexp.MustCompile(`\$\{(ip|ips|public_ip):(role:)?([a-z0-9][a-z0-9_-]*)\}`)

// Addresses is what the expansion knows: machine name → info, role → names.
type Addresses struct {
	byName map[string]provision.MachineInfo
	byRole map[string][]string
}

// NewAddresses indexes the converged infra.
func NewAddresses(infra provision.Infra, infos map[string]provision.MachineInfo) Addresses {
	a := Addresses{byName: infos, byRole: map[string][]string{}}
	for _, name := range infra.Order {
		m := infra.Machines[name]
		a.byRole[m.Spec.Role] = append(a.byRole[m.Spec.Role], name)
	}
	return a
}

// Expand replaces every placeholder in s. An unknown machine or role is an
// error — a config pointing at nothing is a run that fails later in a
// stranger way.
func (a Addresses) Expand(s string) (string, error) {
	var firstErr error
	out := placeholder.ReplaceAllStringFunc(s, func(match string) string {
		sub := placeholder.FindStringSubmatch(match)
		kind, isRole, name := sub[1], sub[2] != "", sub[3]
		var names []string
		if isRole {
			names = a.byRole[name]
			if len(names) == 0 {
				firstErr = firstOf(firstErr, fmt.Errorf("placeholder %s: no machines of role %q", match, name))
				return match
			}
		} else {
			if _, ok := a.byName[name]; !ok {
				firstErr = firstOf(firstErr, fmt.Errorf("placeholder %s: unknown machine %q", match, name))
				return match
			}
			names = []string{name}
		}
		pick := func(n string) string {
			info := a.byName[n]
			if kind == "public_ip" {
				return info.PublicIP
			}
			return info.PrivateIP
		}
		if kind == "ips" {
			vals := make([]string, 0, len(names))
			for _, n := range names {
				vals = append(vals, pick(n))
			}
			return strings.Join(vals, ",")
		}
		return pick(names[0])
	})
	return out, firstErr
}

// ExpandMap expands every value of a map, returning a new map with sorted
// iteration for determinism.
func (a Addresses) ExpandMap(m map[string]string) (map[string]string, error) {
	out := make(map[string]string, len(m))
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v, err := a.Expand(m[k])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", k, err)
		}
		out[k] = v
	}
	return out, nil
}

// ExpandList expands every element of a list.
func (a Addresses) ExpandList(l []string) ([]string, error) {
	if l == nil {
		return nil, nil
	}
	out := make([]string, len(l))
	for i, s := range l {
		v, err := a.Expand(s)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

func firstOf(a, b error) error {
	if a != nil {
		return a
	}
	return b
}
