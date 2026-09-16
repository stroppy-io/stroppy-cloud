package run

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/graphene-ci/pipeline/pkg/pipelinetest"
	"github.com/graphene-ci/pipeline/pkg/ref"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/activities"
	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/provision"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

func TestManagedYDBEndpointAndCleanup(t *testing.T) {
	for _, mode := range []string{"serverless", "dedicated"} {
		t.Run(mode, func(t *testing.T) {
			s := newSim(t)
			wf := pipelinetest.Workflow(s.w, PipelineID, Run)
			run := simRun()
			run.Containers, run.HostPrep, run.Flows = nil, nil, nil
			run.Machines = run.Machines[1:]
			run.Workload.DriverType = "ydb"
			run.Workload.URL = "grpcs://pending:2135/?database=/pending"
			run.Workload.YDBIAMCredentialsSecret = "yc-sa-key"
			run.Workload.Baseline = nil
			run.Workload.Segments = []json.RawMessage{json.RawMessage(`{"name":"smoke","workload":{"script":"simple"},"run":{"duration":"10s"}}`)}
			run.ManagedYDB = &spec.ManagedYDB{Type: mode, LocationID: "ru-central1", StorageSizeLimitGB: 50}
			kind := "DatabaseServerless"
			if mode == "dedicated" {
				kind = "DatabaseDedicated"
				run.ManagedYDB.Zones = []string{"ru-central1-a", "ru-central1-b", "ru-central1-d"}
				run.ManagedYDB.ResourcePresetID = "medium"
				run.ManagedYDB.NodeCount = 3
				run.ManagedYDB.StorageGroups = 1
				run.ManagedYDB.StorageType = "ssd"
			}
			names := provision.NewNames(run.Tenant, run.RunID)
			s.converge(t, run, map[string]string{"runner-1": "10.130.0.20"})
			if mode == "dedicated" {
				for _, zone := range run.ManagedYDB.Zones {
					require.NoError(t, s.objects.After(10*time.Second, ref.OwnerRef(ycSubnetKind+"/"+names.Subnet()+"-"+zone), xpReadyStatus(map[string]any{"id": "subnet-" + zone})))
				}
			}
			db := ref.OwnerRef("k8s.ydb.yandex-cloud.jet.crossplane.io.v1alpha1." + kind + "/" + names.Prefix() + "-ydb")
			endpoint := "grpcs://ydb.example:2135/?database=/ru-central1/test/owned"
			require.NoError(t, s.objects.After(2*time.Minute, db, xpReadyStatus(map[string]any{"id": "owned-ydb", "ydbFullEndpoint": endpoint})))
			s.w.OnAgentActivity(s.agent(run, "runner-1"), activities.NameWaitDatabase, mock.Anything,
				mock.MatchedBy(func(req activities.WaitDatabaseRequest) bool {
					return req.Endpoint == endpoint && req.Workload.YDBIAMCredentialsSecret == "yc-sa-key" && req.Workload.StroppyImage == run.Workload.StroppyImage
				})).Return(activities.WaitDatabaseResult{Ready: true, Address: "10.130.0.21:2135", Attempts: 1, QueryAttempts: 1}, nil).Once()
			s.w.OnAgentActivity(s.agent(run, "runner-1"), activities.NameRunSegment, mock.Anything, mock.MatchedBy(func(req activities.RunSegmentRequest) bool {
				return req.URL == endpoint && req.Workload.YDBIAMCredentialsSecret == "yc-sa-key" && !strings.Contains(req.URL, "pending")
			})).Return(activities.RunSegmentResult{Result: spec.SegmentResult{Name: "smoke", Status: spec.SegmentCompleted, Metrics: map[string]spec.MetricValue{"iterations_total": {Value: 1}}}}, nil).Once()
			s.w.Env.ExecuteWorkflow(wf, run)
			require.NoError(t, s.w.Env.GetWorkflowError())
			s.w.Env.AssertExpectations(t)
			resource, ok := s.w.Resource(db)
			require.True(t, ok)
			require.Equal(t, "deleted", resource.Phase)
			s.w.AssertNoLeaks(t)
		})
	}
}
