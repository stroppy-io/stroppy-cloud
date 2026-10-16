package api

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/audit"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/dbkind"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/observe"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
	"github.com/stroppy-io/stroppy-cloud/internal/oas"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// Every value set the server sends is an enum of the contract, equal to the
// domain's list: a new value is a code change in both, and the SPA (which
// owns every text) translates each enum value (web/src/lib/vocab.test.ts).
func TestVocabulariesMatchTheContract(t *testing.T) {
	var kinds, groups, metricKeys, quotaNames, quotaUnits []string
	for _, d := range dbkind.Databases() {
		kinds = append(kinds, string(d.Kind))
	}
	for _, m := range catalog.Metrics() {
		metricKeys = append(metricKeys, m.Key)
		groups = append(groups, m.Group)
	}
	for _, q := range spec.QuotaKinds {
		quotaNames = append(quotaNames, q.Name)
		quotaUnits = append(quotaUnits, q.Unit)
	}
	var waits []string
	for _, w := range run.WaitReasons {
		waits = append(waits, string(w))
	}
	for _, tc := range []struct {
		name     string
		domain   []string
		contract []string
	}{
		{"DatabaseKind", kinds, values(oas.DatabaseKind("").AllValues())},
		{"TopologyRole", topology.Roles, values(oas.TopologyRole("").AllValues())},
		{"RunEventKind", run.EventKinds, values(oas.RunEventKind("").AllValues())},
		{"RunWaitReason", waits, values(oas.RunWaitReason("").AllValues())},
		{"AuditAction", audit.Actions, values(oas.AuditAction("").AllValues())},
		{"AuditTargetKind", audit.TargetKinds, values(oas.AuditTargetKind("").AllValues())},
		{"MetricKey", metricKeys, values(oas.MetricKey("").AllValues())},
		{"MetricGroup", groups, values(oas.MetricGroup("").AllValues())},
		{"QuotaName", quotaNames, values(oas.QuotaName("").AllValues())},
		{"QuotaUnit", quotaUnits, values(oas.QuotaUnit("").AllValues())},
		{"LogLevel", observe.LogLevels, values(oas.LogLevel("").AllValues())},
		{"ErrorReason", errorReasons(t), values(oas.ErrorReason("").AllValues())},
	} {
		t.Run(tc.name, func(t *testing.T) {
			domain, contract := set(tc.domain), set(tc.contract)
			for v := range domain {
				if !contract[v] {
					t.Errorf("the server sends %q, which the contract's %s does not list", v, tc.name)
				}
			}
			for v := range contract {
				if !domain[v] {
					t.Errorf("the contract's %s lists %q, which the server never sends", tc.name, v)
				}
			}
		})
	}
}

func values[T ~string](in []T) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = string(v)
	}
	return out
}

func set(in []string) map[string]bool {
	out := make(map[string]bool, len(in))
	for _, v := range in {
		out[v] = true
	}
	return out
}

// errorReasons scans the server for the reasons it can send: every
// `.Because("<reason>"` and every `errs.NotFound("<what>")` (as
// `<what>_not_found`), plus the kinds library.owned names with a variable.
func errorReasons(t *testing.T) []string {
	t.Helper()
	because := regexp.MustCompile(`\.Because\("([a-z0-9_]+)"`)
	notFound := regexp.MustCompile(`errs\.NotFound\("([^"]+)"\)`)
	word := regexp.MustCompile(`[^a-z0-9]+`)
	reasons := []string{"database_not_found", "workload_not_found", "test_not_found"}
	err := filepath.WalkDir(filepath.Join(".."), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "oas" {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for _, m := range because.FindAllSubmatch(src, -1) {
			reasons = append(reasons, string(m[1]))
		}
		for _, m := range notFound.FindAllSubmatch(src, -1) {
			reasons = append(reasons, strings.Trim(word.ReplaceAllString(strings.ToLower(string(m[1])), "_"), "_")+"_not_found")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(reasons) < 50 {
		t.Fatalf("found only %d reasons; the scan is broken", len(reasons))
	}
	slices.Sort(reasons)
	return slices.Compact(reasons)
}
