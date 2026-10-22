package run

import (
	"encoding/json"
	"testing"

	"github.com/graphene-ci/pipeline/pkg/ref"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/activities"
	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/provision"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// The Yandex network model in the simulator: the profile network exists
// before the run (stroppy-provider-config made it, or the user did); the
// run declares only its security group and zonal subnets in it.

// forProvider reads the serialized Crossplane parameters of a run object.
func forProvider(t *testing.T, s *sim, kind, name string) map[string]any {
	t.Helper()
	res, ok := s.w.Resource(ref.OwnerRef(kind + "/" + name))
	require.True(t, ok, "%s/%s declared", kind, name)
	var raw any
	require.NoError(t, json.Unmarshal(res.Spec, &raw))
	params := findObject(raw, "forProvider")
	require.NotNil(t, params, "serialized Crossplane spec missing")
	return params
}

func runNoop(t *testing.T, s *sim, run spec.Run) {
	t.Helper()
	wf := pipelinetestWorkflow(s)
	s.converge(t, run, map[string]string{"runner-1": "10.130.0.20"})
	s.w.OnAgentActivity(s.agent(run, "runner-1"), activities.NameRunSegment, mock.Anything, mock.Anything).
		Return(completed("smoke", 10), nil).Once()
	s.w.Env.ExecuteWorkflow(wf, run)
	require.NoError(t, s.w.Env.GetWorkflowError())
}

func TestSimulatedRunJoinsCreatedProfileNetwork(t *testing.T) {
	s := newSim(t)
	run := noopRun()
	run.Network.AllowPublicIPs = true
	run.Network.Subnets = []spec.Subnet{{Zone: "ru-central1-d", CIDR: "10.128.4.16/28"}}
	runNoop(t, s, run)

	names := provision.NewNames(run.Tenant, run.RunID)
	network := spec.ProfileNetworkName(run.Provider.ProviderConfigName)
	group := forProvider(t, s, ycSGKind, names.SecurityGroup())
	require.Equal(t, network, group["networkIdRef"].(map[string]any)["name"])
	intra := group["ingress"].([]any)[0].(map[string]any)
	require.Equal(t, []any{"10.128.4.16/28"}, intra["v4CidrBlocks"], "intra-run traffic is the run's own block only")
	subnet := forProvider(t, s, ycSubnetKind, names.Subnet())
	require.Equal(t, network, subnet["networkIdRef"].(map[string]any)["name"])
	require.Equal(t, []any{"10.128.4.16/28"}, subnet["v4CidrBlocks"])
	require.Nil(t, subnet["routeTableId"], "public machines need no route")

	_, declared := s.w.Resource(ref.OwnerRef(ycNetworkKind + "/" + network))
	require.False(t, declared, "the profile network is not the run's")
	_, declared = s.w.Resource(ref.OwnerRef(ycGatewayKind + "/" + names.Gateway()))
	require.False(t, declared)
	gone, _ := s.w.Resource(ref.OwnerRef(ycSubnetKind + "/" + names.Subnet()))
	require.Equal(t, "deleted", gone.Phase, "the run's subnet goes with the run")
	s.w.AssertNoLeaks(t)
}

func TestSimulatedRunJoinsExistingNetwork(t *testing.T) {
	s := newSim(t)
	run := noopRun()
	run.Provider.Settings = json.RawMessage(`{"cloud_id":"b1glku4lgd6gabcdefgh","folder_id":"b1gia87mbaomkfvsleds","zone":"ru-central1-d","network":{"kind":"existing","network_id":"enp2v5nl4h1mabcdefgh","pool_cidr":"172.20.0.0/20"}}`)
	run.Network.Subnets = []spec.Subnet{{Zone: "ru-central1-d", CIDR: "172.20.0.0/28"}}
	runNoop(t, s, run)

	names := provision.NewNames(run.Tenant, run.RunID)
	for _, params := range []map[string]any{forProvider(t, s, ycSGKind, names.SecurityGroup()), forProvider(t, s, ycSubnetKind, names.Subnet())} {
		require.Equal(t, "enp2v5nl4h1mabcdefgh", params["networkId"])
		require.Nil(t, params["networkIdRef"])
	}
	s.w.AssertNoLeaks(t)
}

func TestSimulatedPrivateRunUsesProfileRoutes(t *testing.T) {
	s := newSim(t)
	s.sharedRoutes = "enp-profile-routes"
	run := noopRun()
	run.Provider.Settings = json.RawMessage(`{"cloud_id":"b1glku4lgd6gabcdefgh","folder_id":"b1gia87mbaomkfvsleds","zone":"ru-central1-d","public_ips":false,"network":{"kind":"create"}}`)
	run.Network.AllowPublicIPs = false
	runNoop(t, s, run)

	names := provision.NewNames(run.Tenant, run.RunID)
	subnet := forProvider(t, s, ycSubnetKind, names.Subnet())
	require.Equal(t, "enp-profile-routes", subnet["routeTableId"], "private egress goes through the profile's NAT")
	for _, kind := range []string{ycGatewayKind, ycRouteTableKind} {
		_, declared := s.w.Resource(ref.OwnerRef(kind + "/" + names.Gateway()))
		require.False(t, declared, "no run-owned gateway on a profile network with a route out")
		_, declared = s.w.Resource(ref.OwnerRef(kind + "/" + names.RouteTable()))
		require.False(t, declared)
	}
	s.w.AssertNoLeaks(t)
}

func TestSimulatedPrivateRunWithoutProfileRoutesFails(t *testing.T) {
	s := newSim(t)
	run := noopRun()
	run.Provider.Settings = json.RawMessage(`{"cloud_id":"b1glku4lgd6gabcdefgh","folder_id":"b1gia87mbaomkfvsleds","zone":"ru-central1-d","public_ips":false,"network":{"kind":"create"}}`)
	run.Network.AllowPublicIPs = false
	s.w.Env.ExecuteWorkflow(pipelinetestWorkflow(s), run)
	require.ErrorContains(t, s.w.Env.GetWorkflowError(), "no route table id")
	names := provision.NewNames(run.Tenant, run.RunID)
	_, declared := s.w.Resource(ref.OwnerRef(ycSubnetKind + "/" + names.Subnet()))
	require.False(t, declared, "no subnet without the profile's route out")
	s.w.AssertNoLeaks(t)
}

func TestSimulatedRunWithoutAllocationDeclaresNothing(t *testing.T) {
	s := newSim(t)
	run := noopRun()
	run.Network.Subnets = nil
	s.w.Env.ExecuteWorkflow(pipelinetestWorkflow(s), run)
	require.ErrorContains(t, s.w.Env.GetWorkflowError(), "no subnet allocated")
	names := provision.NewNames(run.Tenant, run.RunID)
	_, declared := s.w.Resource(ref.OwnerRef(ycSGKind + "/" + names.SecurityGroup()))
	require.False(t, declared, "no fallback range on a shared network")
	require.Equal(t, "failure", s.w.Outcome(runOwner()))
}
