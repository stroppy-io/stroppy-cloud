// Package provision turns the machine part of a RunSpec into cloud
// resources through Crossplane — one Provider per cloud, all producing the
// same Infra: a root network handle (the thing ToStand moves), one
// crossplane VM per machine, and the agent record each VM carries.
//
// Every resource is a k8slib.Resource: an entity in the run's ownership
// tree with apply + converge, drift heal and delete. Cross-resource wiring
// uses the provider's native *Ref fields, so no cloud id ever travels
// through workflow history. Readiness is the crossplane Ready condition,
// tightened per kind where the condition alone lies (an Instance is ready
// when it RUNS).
package provision

import (
	"encoding/json"
	"fmt"
	"strings"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	corev1 "k8s.io/api/core/v1"

	k8slib "github.com/graphene-ci/library/k8s"
	"github.com/graphene-ci/pipeline/pkg/pipeline"

	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/topo"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// MachineInfo is what a converged VM reports back.
type MachineInfo struct {
	// ID is the provider's own id of the instance.
	ID string
	// PrivateIP is the address inside the run network.
	PrivateIP string
	// PublicIP is the NAT/public address, when allowed.
	PublicIP string
}

// Machine is one provisioned VM with its agent.
type Machine struct {
	Spec  spec.Machine
	Agent pipeline.AgentHandle
	// VM is the crossplane instance handle (a tree node under the network).
	VM pipeline.Handle
	// wait blocks until the instance runs and reads its observed addresses.
	wait func(ctx pipeline.Context) (MachineInfo, error)
}

// Ready blocks until the VM runs and returns its addresses; a failure fails
// the run (the SDK convention).
func (m *Machine) Ready(ctx pipeline.Context) MachineInfo {
	info, err := m.TryReady(ctx)
	if err != nil {
		panic(fmt.Errorf("machine %s: %w", m.Spec.Name, err))
	}
	return info
}

// TryReady is Ready with the error in hand.
func (m *Machine) TryReady(ctx pipeline.Context) (MachineInfo, error) {
	if m.wait == nil {
		return MachineInfo{}, nil
	}
	return m.wait(ctx)
}

// Infra is the provisioned network and its machines.
type Infra struct {
	managedEndpoint func(pipeline.Context) (string, error)
	// Root is the resource ToStand moves: the network; everything else is
	// its descendant.
	Root pipeline.Handle
	// Machines by spec name, in spec order (Order).
	Machines map[string]*Machine
	Order    []string
}

// ManagedEndpoint waits for the run-owned managed database, if any.
func (i Infra) ManagedEndpoint(ctx pipeline.Context) (string, error) {
	if i.managedEndpoint == nil {
		return "", nil
	}
	return i.managedEndpoint(ctx)
}

// Machine returns the machine by name.
func (i Infra) Machine(name string) (*Machine, bool) {
	m, ok := i.Machines[name]
	return m, ok
}

// ByRole lists machines of one role in spec order.
func (i Infra) ByRole(role string) []*Machine {
	var out []*Machine
	for _, name := range i.Order {
		if m := i.Machines[name]; m.Spec.Role == role {
			out = append(out, m)
		}
	}
	return out
}

// Provider provisions the machines of a run in one cloud.
type Provider interface {
	Kind() spec.ProviderKind
	// Scheme teaches a k8s client the provider's CRD types.
	Scheme() k8slib.ClientOption
	// Provision declares the network and every machine. agents are the
	// pre-declared agent records (one per machine name) whose CloudInit
	// goes into the VM user-data. Nothing blocks here: declarations return
	// handles; Infra machines wait through Ready.
	Provision(ctx pipeline.Context, k8s *k8slib.Client, run spec.Run, agents map[string]pipeline.AgentHandle) (Infra, error)
	// Record declares one object of every kind the provider uses, so the
	// recording pass registers their entity workflows even when the
	// zero-value spec has no machines.
	Record(ctx pipeline.Context, k8s *k8slib.Client)
}

// Registry maps kinds to providers.
type Registry map[spec.ProviderKind]Provider

// Default is every provider the pipeline ships.
func Default() Registry {
	return Registry{
		spec.ProviderYandex: Yandex{},
		spec.ProviderAWS:    AWS{},
	}
}

// For picks the provider of a run.
func (r Registry) For(kind spec.ProviderKind) (Provider, error) {
	p, ok := r[kind]
	if !ok {
		return nil, fmt.Errorf("provision: unsupported provider %q", kind)
	}
	return p, nil
}

// Names derives the resource names of one run: unique across runs of a
// namespace (entity ids) and across the cluster (cluster-scoped managed
// resources), readable in the cloud console.
type Names struct {
	prefix string
}

// NewNames builds the naming scheme of a run.
func NewNames(tenant, runID string) Names {
	short := runID
	if len(short) > 8 {
		short = short[:8]
	}
	return Names{prefix: "stroppy-" + topo.Slug(tenant, 20) + "-" + topo.Slug(short, 8)}
}

// Prefix is the common prefix.
func (n Names) Prefix() string { return n.prefix }

// Network names the run network.
func (n Names) Network() string { return n.prefix + "-net" }

// Subnet names the run subnet.
func (n Names) Subnet() string { return n.prefix + "-subnet" }

// SecurityGroup names the run security group.
func (n Names) SecurityGroup() string { return n.prefix + "-sg" }

// Gateway names the internet gateway (aws).
func (n Names) Gateway() string { return n.prefix + "-igw" }

// RouteTable names the route table (aws).
func (n Names) RouteTable() string { return n.prefix + "-rt" }

// Machine names one VM.
func (n Names) Machine(machine string) string { return n.prefix + "-" + topo.Slug(machine, 24) }

// Disk names one secondary disk of a machine.
func (n Names) Disk(machine, disk string) string {
	return n.Machine(machine) + "-" + topo.Slug(disk, 12)
}

// Rule names one security-group rule (aws).
func (n Names) Rule(kind string, i int) string { return fmt.Sprintf("%s-%s-%d", n.prefix, kind, i) }

// xpReady is the readiness of a crossplane managed resource: its Ready
// condition is True. For a beat after apply an MR carries no conditions at
// all; the library default (no conditions → ready) would latch such a
// record ready before the cloud object exists, so every resource carries
// this override.
func xpReady(c xpv1.Condition) bool { return c.Status == corev1.ConditionTrue }

// providerRef points a managed resource at the tenant's ProviderConfig.
func providerRef(name string) xpv1.ResourceSpec {
	return xpv1.ResourceSpec{ProviderConfigReference: &xpv1.Reference{Name: name}}
}

// ptr is the small price of foreign specs made of pointers.
func ptr[T any](v T) *T { return &v }

// strPtrs maps strings to pointer slices, the shape provider lists use.
func strPtrs(ss ...string) []*string {
	out := make([]*string, 0, len(ss))
	for _, s := range ss {
		out = append(out, ptr(s))
	}
	return out
}

// decodeSettings parses the provider settings JSON of a run into the
// provider's own struct; an empty settings blob is an error — the server
// always sends a baked value.
func decodeSettings[T any](raw json.RawMessage) (T, error) {
	var out T
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return out, fmt.Errorf("provision: empty provider settings")
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("provision: settings: %w", err)
	}
	return out, nil
}

// labels are the provider labels every resource of a run carries; the
// run and tenant are always there so a console shows what a thing is.
func labels(run spec.Run, extra map[string]string) map[string]*string {
	out := map[string]*string{
		"stroppy-run":    ptr(topo.Slug(run.RunID, 63)),
		"stroppy-tenant": ptr(topo.Slug(run.Tenant, 63)),
	}
	for _, k := range topo.SortedKeys(extra) {
		out[topo.Slug(k, 63)] = ptr(extra[k])
	}
	return out
}

// intraCIDR is the run network itself: every machine may talk to every
// other on every port (the benchmark topology decides the real flows; a
// security group per flow is a later refinement).
func intraCIDR(run spec.Run) string {
	if run.Network.CIDR != "" {
		return run.Network.CIDR
	}
	return "10.130.0.0/24"
}
