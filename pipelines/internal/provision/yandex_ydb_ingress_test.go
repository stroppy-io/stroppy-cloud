package provision

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

func TestManagedYDBHealthIngressScope(t *testing.T) {
	subnets := []yandexSubnet{{"ru-central1-a", "a", "10.128.0.0/28"}, {"ru-central1-b", "b", "10.128.0.16/28"}}
	for _, kind := range []string{"", "serverless", "dedicated"} {
		t.Run(kind, func(t *testing.T) {
			run := spec.Run{}
			if kind != "" {
				run.ManagedYDB = &spec.ManagedYDB{Type: kind}
			}
			rules := yandexIngress(run, subnets)
			want := 1
			if kind == "dedicated" {
				want = 2
			}
			if len(rules) != want {
				t.Fatalf("got %d rules, want %d", len(rules), want)
			}
			// Intra-run traffic is the run's own blocks, never the shared
			// network: other runs in it stay out.
			if len(rules[0].V4CidrBlocks) != 2 || *rules[0].V4CidrBlocks[0] != "10.128.0.0/28" || *rules[0].V4CidrBlocks[1] != "10.128.0.16/28" {
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
