package run

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/uitext"
)

// EventTitleKey is how the SPA names an event's title
// (web/src/components/runs/events/eventTitle.ts): dots and dashes become
// underscores and Graphene's American "canceled" reads "cancelled".
func eventTitleKey(kind string) string {
	k := strings.NewReplacer(".", "_", "-", "_").Replace(kind)
	return "runs.events.titles." + strings.ReplaceAll(k, "canceled", "cancelled")
}

// Every event kind a run's timeline shows — the pipeline's milestones and
// Graphene's run lifecycle the projection reads — has a title in the SPA.
func TestEventKindsHaveUIText(t *testing.T) {
	kinds := map[string]string{}
	milestones, err := os.ReadFile(filepath.Join("..", "..", "..", "pipelines", "internal", "events", "events.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range regexp.MustCompile(`=\s*"([a-z]+\.[a-z_]+)"`).FindAllSubmatch(milestones, -1) {
		kinds[string(m[1])] = "pipelines/internal/events"
	}
	projection, err := os.ReadFile("projection.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range regexp.MustCompile(`"(run-[a-z-]+)"`).FindAllSubmatch(projection, -1) {
		kinds[string(m[1])] = "projection.go"
	}
	if len(kinds) < 15 {
		t.Fatalf("found only %d event kinds; the scan is broken", len(kinds))
	}
	for kind, from := range kinds {
		for _, lang := range uitext.Langs {
			if ok, err := uitext.Has(lang, eventTitleKey(kind)); err != nil || !ok {
				t.Errorf("%s: event %q (%s) has no %s (%v)", lang, kind, from, eventTitleKey(kind), err)
			}
		}
	}
}
