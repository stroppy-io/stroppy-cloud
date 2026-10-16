package provision

import (
	"errors"
	"testing"

	"github.com/graphene-ci/pipeline/pkg/pipeline"
)

// A refused network fails the machine with that refusal, and nothing after
// it in the chain is waited for.
func TestUpstreamStopsAtTheRefusal(t *testing.T) {
	refused := errors.New("Network/run-net: create failed: ResourceExhausted")
	var waited []string
	step := func(name string, err error) readyFn {
		return func(pipeline.Context) error { waited = append(waited, name); return err }
	}
	err := upstream(pipeline.Context{}, step("network", refused), step("group", nil), step("subnet", nil))
	if !errors.Is(err, refused) {
		t.Fatalf("err = %v, want the network refusal", err)
	}
	if len(waited) != 1 {
		t.Fatalf("waited for %v after the refusal", waited)
	}
	waited = nil
	if err := upstream(pipeline.Context{}, step("network", nil), step("group", nil)); err != nil || len(waited) != 2 {
		t.Fatalf("ready chain: err=%v waited=%v", err, waited)
	}
}
