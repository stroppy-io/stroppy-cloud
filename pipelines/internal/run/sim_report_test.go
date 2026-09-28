package run

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/graphene-ci/pipeline/pkg/ref"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/activities"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

func TestSimulatedNativeReportArtifact(t *testing.T) {
	for _, upload := range []bool{true, false} {
		t.Run(map[bool]string{true: "published", false: "upload-failed"}[upload], func(t *testing.T) {
			s := newSim(t)
			run := noopRun()
			wf := pipelinetestWorkflow(s)
			s.converge(t, run, map[string]string{"runner-1": "10.130.0.20"})
			runner := s.agent(run, "runner-1")
			res := completed("smoke", 3)
			res.LogPath = ""
			res.ReportPath = "/ws/stroppy/smoke/report.json"
			res.Result.Report = json.RawMessage(`{"schema":1,"unknown":{"large":18446744073709551615}}`)
			if !upload {
				res.Result.Report = nil
				res.Result.ReportOmitted = "report exceeds inline limit; see report artifact"
			}
			s.w.OnAgentActivity(runner, activities.NameRunSegment, mock.Anything, mock.Anything).Return(res, nil).Once()
			if upload {
				s.w.File(runner, res.ReportPath, res.Result.Report)
			}
			s.w.Env.ExecuteWorkflow(wf, run)
			require.NoError(t, s.w.Env.GetWorkflowError())
			var result spec.Result
			require.NoError(t, s.w.Env.GetWorkflowResult(&result))
			require.Equal(t, res.Result.Report, result.Segments[0].Report)
			if upload {
				name := "artifact/8f1c3f2a-stroppy-smoke-report"
				require.Equal(t, name, result.Segments[0].ReportArtifact)
				require.Contains(t, result.Artifacts, name)
				item, ok := s.w.Resource(ref.OwnerRef(name))
				require.True(t, ok)
				require.Equal(t, "ready", item.Phase)
				require.True(t, s.w.Env.Now().Add(logArtifactRetention).Equal(item.KeepUntil))
				s.w.Advance(logArtifactRetention + time.Hour)
				item, _ = s.w.Resource(ref.OwnerRef(name))
				require.Equal(t, "deleted", item.Phase)
			} else {
				require.Empty(t, result.Segments[0].ReportArtifact)
				require.Contains(t, result.Segments[0].ReportOmitted, "artifact upload failed")
			}
			s.w.AssertNoLeaks(t)
		})
	}
}
