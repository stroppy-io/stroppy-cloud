package observe

import (
	"context"
	"regexp"
	"sync"
	"time"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
)

// stroppy logs every step of a workload's preparation (bench/step.go):
// "Start of 'load_data' step", "End of 'load_data' step (took 5m57s)". The
// load itself logs no step: past the last "End of" the segment is under load.
var stepMarker = regexp.MustCompile(`"msg":"(Start|End) of '([^']+)' step`)

const (
	stageQuery = `"bench/step.go"`
	// stageFresh is how long a read stage is reused: run lists poll every
	// few seconds, a stage changes in minutes.
	stageFresh   = 15 * time.Second
	stageTimeout = 3 * time.Second
)

type stageEntry struct {
	at    time.Time
	stage run.Stage
}

// stages caches the stage read per run.
type stages struct {
	mu   sync.Mutex
	runs map[string]stageEntry
}

// StageOf reads where the running segment's stroppy is from its last step
// marker; zero when the run is not under its workload, the segment logged
// no step yet, or the log store does not answer in time.
func (s *Service) StageOf(ctx context.Context, r run.Run) run.Stage {
	if s.logs == nil || r.Status != run.StatusRunning || r.Phase != run.PhaseWorkload {
		return run.Stage{}
	}
	var since time.Time
	for _, seg := range r.State.Segments {
		if seg.StartedAt != nil && seg.FinishedAt == nil {
			since = *seg.StartedAt
		}
	}
	if since.IsZero() {
		return run.Stage{}
	}
	key := r.ID.String()
	now := time.Now()
	s.stages.mu.Lock()
	if e, ok := s.stages.runs[key]; ok && now.Sub(e.at) < stageFresh {
		s.stages.mu.Unlock()
		return e.stage
	}
	s.stages.mu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, stageTimeout)
	defer cancel()
	page, err := s.logs.Raw(ctx, ScopeOf(r), stageQuery, since, now, 1)
	if err != nil {
		return run.Stage{}
	}
	stage := stageOfLines(page.Lines)
	s.stages.mu.Lock()
	if s.stages.runs == nil {
		s.stages.runs = map[string]stageEntry{}
	}
	for k, e := range s.stages.runs {
		if now.Sub(e.at) > time.Hour {
			delete(s.stages.runs, k)
		}
	}
	s.stages.runs[key] = stageEntry{at: now, stage: stage}
	s.stages.mu.Unlock()
	return stage
}

// stageOfLines reads the stage from the newest step marker.
func stageOfLines(lines []LogLine) run.Stage {
	for _, l := range lines {
		m := stepMarker.FindStringSubmatch(l.Message)
		if m == nil {
			continue
		}
		kind := run.StagePreparing
		if m[1] == "End" {
			kind = run.StageLoad
		}
		return run.Stage{Kind: kind, Step: m[2], Since: l.Time}
	}
	return run.Stage{}
}
