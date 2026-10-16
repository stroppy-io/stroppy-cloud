package cloud

import (
	"slices"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// Every quota a provider reports is a spec.QuotaKind with the same unit:
// the server contract names exactly those.
func TestReportedQuotasAreQuotaKinds(t *testing.T) {
	has := func(name, unit string) bool {
		return slices.Contains(spec.QuotaKinds, spec.QuotaKind{Name: name, Unit: unit})
	}
	for _, q := range yandexQuotaIDs {
		if !has(q.id, q.unit) {
			t.Errorf("yandex quota %s (%s) is not a spec.QuotaKind", q.id, q.unit)
		}
	}
	for _, q := range awsQuotaCodes {
		if !has(q.name, q.unit) {
			t.Errorf("aws quota %s (%s) is not a spec.QuotaKind", q.name, q.unit)
		}
	}
}
