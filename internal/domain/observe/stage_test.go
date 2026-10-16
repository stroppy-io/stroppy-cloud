package observe

import (
	"testing"
	"time"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
)

func TestStageOfLines(t *testing.T) {
	at := time.Date(2026, 10, 16, 14, 16, 58, 0, time.UTC)
	line := func(msg string) LogLine {
		return LogLine{Time: at, Message: `{"level":"info","logger":"workload","caller":"bench/step.go:154","msg":"` + msg + `","workload":"tpcc/tx"}`}
	}
	for _, c := range []struct {
		name string
		in   []LogLine
		want run.Stage
	}{
		{"preparing", []LogLine{line("Start of 'validate_population' step")}, run.Stage{Kind: run.StagePreparing, Step: "validate_population", Since: at}},
		{"under load past the last step", []LogLine{line("End of 'validate_population' step (took 21m25s)")}, run.Stage{Kind: run.StageLoad, Step: "validate_population", Since: at}},
		{"no marker", []LogLine{{Time: at, Message: "something else"}}, run.Stage{}},
		{"nothing", nil, run.Stage{}},
	} {
		if got := stageOfLines(c.in); got != c.want {
			t.Errorf("%s: %+v; want %+v", c.name, got, c.want)
		}
	}
}
