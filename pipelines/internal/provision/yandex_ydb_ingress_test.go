package provision

import (
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
	"testing"
)

func TestManagedYDBHealthIngressScope(t *testing.T) {
	for _, kind := range []string{"", "serverless", "dedicated"} {
		t.Run(kind, func(t *testing.T) {
			run := spec.Run{Network: spec.Network{CIDR: "10.130.0.0/24"}}
			if kind != "" {
				run.ManagedYDB = &spec.ManagedYDB{Type: kind}
			}
			rules := yandexIngress(run)
			want := 1
			if kind == "dedicated" {
				want = 2
			}
			if len(rules) != want {
				t.Fatalf("got %d rules, want %d", len(rules), want)
			}
			if len(rules[0].V4CidrBlocks) != 1 || *rules[0].V4CidrBlocks[0] != "10.130.0.0/24" {
				t.Fatal("intra-run boundary changed")
			}
			if kind == "dedicated" {
				health := rules[1]
				if health.PredefinedTarget == nil || *health.PredefinedTarget != "loadbalancer_healthchecks" || health.Protocol == nil || *health.Protocol != "TCP" || health.Port == nil || *health.Port != 2135 || len(health.V4CidrBlocks) != 0 {
					t.Fatal("health check rule is broader than the managed YDB service")
				}
			}
		})
	}
}
