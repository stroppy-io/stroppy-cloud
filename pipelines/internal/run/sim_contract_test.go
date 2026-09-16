package run

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/graphene-ci/pipeline/pkg/id"
	"github.com/graphene-ci/pipeline/pkg/ref"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/provision"

	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/activities"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

func TestSimulatedWarmupAndArtifactInput(t *testing.T) {
	s := newSim(t)
	wf := pipelinetestWorkflow(s)
	r := simRun()
	r.Machines = r.Machines[1:]
	r.Containers = nil
	r.HostPrep = nil
	r.Flows = nil
	r.Workload.Baseline = nil
	r.Workload.DriverType = "noop"
	r.Workload.URL = "noop://localhost"
	r.Workload.Segments = []json.RawMessage{json.RawMessage(`{"name":"query","workload":{"script":"execute_sql","sql_file":"sql/query.sql"},"run":{"executor":"shared-iterations","iterations":1},"warmup":"10m","files":[{"name":"sql/query.sql","ref":"artifact/query-input"}]}`)}
	input := s.w.SeedArtifact(id.ArtifactId("query-input"), []byte("--= q\nSELECT 1"))
	s.converge(t, r, map[string]string{"runner-1": "10.130.0.20"})
	start := s.w.Env.Now()
	s.w.OnAgentActivity(s.agent(r, "runner-1"), activities.NameRunSegment, mock.Anything, mock.MatchedBy(func(req activities.RunSegmentRequest) bool {
		return req.FileBlobs["sql/query.sql"] == input.Blob.Location
	})).Run(func(mock.Arguments) { require.GreaterOrEqual(t, s.w.Env.Now().Sub(start), 10*time.Minute) }).Return(activities.RunSegmentResult{Result: spec.SegmentResult{Name: "query", Status: spec.SegmentCompleted}}, nil).Once()
	s.w.Env.ExecuteWorkflow(wf, r)
	require.NoError(t, s.w.Env.GetWorkflowError())
}

// Public inputs are rejected before credential setup or cloud declarations.
func TestSimulatedInputContractRejectsBeforeInfra(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		change     func(*spec.Run)
	}{
		{"capacity-before-infra", "storage budget", func(r *spec.Run) {
			*r = simRun()
			r.Workload.DriverType = "postgres"
			r.Workload.Segments = []json.RawMessage{json.RawMessage(`{"name":"huge","workload":{"script":"tpcc/tx","scale_factor":100000},"run":{"duration":"1m","vus":1}}`)}
		}},
		{"cycle", "container dependencies", func(r *spec.Run) {
			r.Containers = []spec.Container{{Name: "a", Role: "runner", Machine: "runner-1", Image: "example/db:1", DependsOn: []string{"b"}}, {Name: "b", Role: "runner", Machine: "runner-1", Image: "example/db:1", DependsOn: []string{"a"}}}
		}},
		{"unknown-driver-field", "workload.driver", func(r *spec.Run) { r.Workload.Driver = map[string]any{"sslmode": "require"} }},
		{"multiple-runners", "exactly one runner", func(r *spec.Run) { m := r.Machines[0]; m.Name = "runner-2"; r.Machines = append(r.Machines, m) }},
		{"unknown-step", "step must be an executable Stroppy step", func(r *spec.Run) {
			r.Workload.Segments = []json.RawMessage{json.RawMessage(`{"name":"bad","workload":{"script":"simple"},"run":{"executor":"shared-iterations","iterations":1},"steps":["workload_queries"]}`)}
		}},
		{"overlapping-files", "cannot overwrite runtime files", func(r *spec.Run) {
			r.Workload.Segments = []json.RawMessage{json.RawMessage(`{"name":"bad","workload":{"script":"simple"},"run":{"executor":"shared-iterations","iterations":1},"files":[{"name":"ca.pem/nested","content":"x"}]}`)}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newSim(t)
			r := noopRun()
			tc.change(&r)
			wf := pipelinetestWorkflow(s)
			s.w.Env.ExecuteWorkflow(wf, r)
			require.ErrorContains(t, s.w.Env.GetWorkflowError(), tc.want)
			require.Empty(t, s.configs)
			require.Empty(t, s.w.Events())
		})
	}
}

func TestSimulatedMachineSettingsReachCrossplane(t *testing.T) {
	s := newSim(t)
	wf := pipelinetestWorkflow(s)
	r := noopRun()
	r.Network.AllowPublicIPs = true
	r.Provider.Settings = json.RawMessage(`{"cloud_id":"b1glku4lgd6gabcdefgh","folder_id":"b1gia87mbaomkfvsleds","zone":"ru-central1-d","preemptible":true,"network":{"kind":"create"}}`)
	no := false
	r.Machines[0].PublicIP = &no
	r.Machines[0].Preemptible = &no
	r.Machines[0].CoreFraction = 50
	r.Machines[0].BootDisk = &spec.BootDisk{GB: 80, Type: "network-ssd", BlockSize: 8192}
	r.Machines[0].Disks = []spec.Disk{{Name: "scratch", GB: 93, Type: "network-ssd-nonreplicated", Mount: "/scratch", Filesystem: "xfs"}}
	s.converge(t, r, map[string]string{"runner-1": "10.130.0.20"})
	s.w.OnAgentActivity(s.agent(r, "runner-1"), activities.NameRunSegment, mock.Anything, mock.Anything).Return(activities.RunSegmentResult{Result: spec.SegmentResult{Name: "smoke", Status: spec.SegmentCompleted}}, nil).Once()
	s.w.Env.ExecuteWorkflow(wf, r)
	require.NoError(t, s.w.Env.GetWorkflowError())
	names := provision.NewNames(r.Tenant, r.RunID)
	vm, ok := s.w.Resource(ref.OwnerRef(ycInstanceKind + "/" + names.Machine("runner-1")))
	require.True(t, ok)
	var raw any
	require.NoError(t, json.Unmarshal(vm.Spec, &raw))
	params := findObject(raw, "forProvider")
	require.NotNil(t, params, "serialized Crossplane spec missing")
	require.Equal(t, float64(50), params["resources"].([]any)[0].(map[string]any)["coreFraction"])
	boot := params["bootDisk"].([]any)[0].(map[string]any)["initializeParams"].([]any)[0].(map[string]any)
	require.Equal(t, float64(80), boot["size"])
	require.Equal(t, float64(8192), boot["blockSize"])
	require.Equal(t, false, params["schedulingPolicy"].([]any)[0].(map[string]any)["preemptible"])
	require.Equal(t, false, params["networkInterface"].([]any)[0].(map[string]any)["nat"])
	_, ok = s.w.Resource(ref.OwnerRef(ycGatewayKind + "/" + names.Gateway()))
	require.True(t, ok, "private VM needs NAT egress")
	subnet, _ := s.w.Resource(ref.OwnerRef(ycSubnetKind + "/" + names.Subnet()))
	require.NoError(t, json.Unmarshal(subnet.Spec, &raw))
	require.Equal(t, "routes", findObject(raw, "forProvider")["routeTableId"])
	s.w.AssertNoLeaks(t)
}

func findObject(value any, key string) map[string]any {
	switch v := value.(type) {
	case map[string]any:
		if x, ok := v[key].(map[string]any); ok {
			return x
		}
		for _, x := range v {
			if found := findObject(x, key); found != nil {
				return found
			}
		}
	case []any:
		for _, x := range v {
			if found := findObject(x, key); found != nil {
				return found
			}
		}
	}
	return nil
}
