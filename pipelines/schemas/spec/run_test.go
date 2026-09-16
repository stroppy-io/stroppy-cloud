package spec

import (
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

const runID = "8f1c3f2a-0000-4000-8000-000000000001"

func machine(name, role string) map[string]any {
	return map[string]any{
		"name":          name,
		"role":          role,
		"cpu":           int64(8),
		"memory_gb":     int64(32),
		"image":         "ubuntu-2404-lts",
		"location":      "ru-central1-d",
		"instance_type": "standard-v3",
	}
}

func baseRun() map[string]any {
	return map[string]any{
		"run_id": runID,
		"tenant": "acme",
		"provider": map[string]any{
			"kind":                 "yandex",
			"settings":             map[string]any{"cloud_id": "b1glku4lgd6gabcdefgh", "folder_id": "b1gia87mbaomkfvsleds", "network": map[string]any{"kind": "create"}},
			"credentials_secret":   "yc-sa-key",
			"provider_config_name": "t-acme",
		},
		"network":  map[string]any{"cidr": "10.130.0.0/24"},
		"machines": []any{machine("db-1", "db"), machine("runner-1", "runner")},
		"workload": map[string]any{
			"runner_role":   "runner",
			"stroppy_image": "ghcr.io/stroppy-io/stroppy:v6.0.0.62",
			"driver_type":   "postgres",
			"url":           "postgres://u:p@${ip:role:db}:5432/db",
			"driver":        map[string]any{"bulkSize": int64(5000)},
			"segments":      []any{map[string]any{"name": "load", "workload": map[string]any{"script": "simple"}, "run": map[string]any{"executor": "shared-iterations", "iterations": int64(1)}}},
			"baseline":      map[string]any{"enabled": true, "tiers": []any{"noop"}},
		},
	}
}

// withContainer returns a base run carrying one container built from over.
func withContainer(over map[string]any) map[string]any {
	c := map[string]any{
		"name": "postgres", "role": "db", "machine": "db-1", "image": "docker.io/library/postgres:17",
	}
	for k, v := range over {
		c[k] = v
	}

	v := baseRun()
	v["containers"] = []any{c}

	return v
}

//nolint:funlen // one table of run values: every valid shape and every rule violation
func TestRun(t *testing.T) {
	full := baseRun()
	full["containers"] = []any{
		map[string]any{
			"name":    "postgres",
			"role":    "db",
			"machine": "db-1",
			"image":   "docker.io/library/postgres:17",
			"env":     map[string]any{"POSTGRES_PASSWORD": "x"},
			"ports":   []any{map[string]any{"container": int64(5432), "host": int64(5432)}},
			"files": []any{map[string]any{
				"path": "/etc/postgresql/postgresql.conf", "content": "shared_buffers = 8GB\n", "mode": "0644",
			}},
			// interval is a nested duration: it arrives from the API in its
			// string form and is coerced inside the list item.
			"healthcheck": map[string]any{"cmd": []any{"pg_isready"}, "interval": "10s", "retries": int64(30)},
			"ulimits":     map[string]any{"nofile": int64(65536)},
		},
	}
	full["host_prep"] = []any{map[string]any{"role": "db", "kind": "sysctl", "content": "vm.swappiness=1\n"}}
	full["scrapes"] = []any{map[string]any{"role": "db", "url": "http://127.0.0.1:9187/metrics", "job": "postgres"}}
	full["flows"] = []any{
		map[string]any{"from_role": "runner", "to_role": "db", "protocol": "tcp", "port": int64(5432)},
		map[string]any{"from_role": "db", "external": "registry.stroppy.io", "protocol": "http", "port": int64(443)},
	}
	full["observability"] = map[string]any{"otlp_endpoint": "otel:4317", "labels": map[string]any{"run": runID}}
	full["keep"] = "2h"
	full["result_expectations"] = []any{"tps", "latency_p99_ms"}

	badMachine := baseRun()
	badMachine["containers"] = []any{map[string]any{
		"name": "postgres", "role": "db", "machine": "nowhere", "image": "postgres:17",
	}}

	dupNames := baseRun()
	dupNames["machines"] = []any{machine("db-1", "db"), machine("db-1", "runner")}

	dupContainers := baseRun()
	dupContainers["containers"] = []any{
		map[string]any{"name": "postgres", "role": "db", "machine": "db-1", "image": "postgres:17"},
		map[string]any{"name": "postgres", "role": "db", "machine": "db-1", "image": "postgres:17"},
	}

	badFlow := baseRun()
	badFlow["flows"] = []any{map[string]any{
		"from_role": "runner", "to_role": "db", "external": "x", "protocol": "tcp", "port": int64(5432),
	}}

	badFlowFrom := baseRun()
	badFlowFrom["flows"] = []any{map[string]any{
		"from_role": "ghost", "to_role": "db", "protocol": "tcp", "port": int64(5432),
	}}

	badFlowTo := baseRun()
	badFlowTo["flows"] = []any{map[string]any{
		"from_role": "runner", "to_role": "ghost", "protocol": "tcp", "port": int64(5432),
	}}

	badScrape := baseRun()
	badScrape["scrapes"] = []any{map[string]any{
		"role": "ghost", "url": "http://127.0.0.1:9187/metrics", "job": "postgres",
	}}

	badHostPrep := baseRun()
	badHostPrep["host_prep"] = []any{map[string]any{"role": "ghost", "kind": "sysctl", "content": "vm.swappiness=1\n"}}

	badRunner := baseRun()
	badRunner["workload"] = map[string]any{
		"runner_role":   "ghost",
		"stroppy_image": "ghcr.io/stroppy-io/stroppy:v6.0.0.62",
		"driver_type":   "postgres",
		"url":           "postgres://u:p@10.0.0.1:5432/db",
		"segments":      []any{map[string]any{"name": "load"}},
	}

	badID := baseRun()
	badID["run_id"] = "not-a-uuid"

	badPort := baseRun()
	badPort["network"] = map[string]any{
		"cidr":    "10.130.0.0/24",
		"ingress": []any{map[string]any{"port": int64(0), "cidr": "0.0.0.0/0"}},
	}

	badLabel := baseRun()
	badLabel["machines"] = []any{
		func() map[string]any {
			m := machine("db-1", "db")
			m["labels"] = map[string]any{"team": strings.Repeat("y", 300)}

			return m
		}(),
		machine("runner-1", "runner"),
	}

	schematest.Run(t, Run(), schematest.Cases{
		Valid: []map[string]any{baseRun(), full},
		Invalid: []schematest.Invalid{
			{Value: badID, Code: "FORMAT_MISMATCH", Path: "run_id"},
			{Value: badPort, Code: "GTE_VIOLATED", Path: "network.ingress[0].port"},
			// Map values carry their own constraints; the path names the key.
			{Value: badLabel, Code: "MAX_LEN_VIOLATED", Path: "machines[0].labels.team"},
			{
				Value: withContainer(map[string]any{"env": map[string]any{"BIG": strings.Repeat("x", 5000)}}),
				Code:  "MAX_LEN_VIOLATED", Path: "containers[0].env.BIG",
			},
			{
				Value: withContainer(map[string]any{"ulimits": map[string]any{"nofile": int64(-2)}}),
				Code:  "GTE_VIOLATED", Path: "containers[0].ulimits.nofile",
			},
			// A nested duration below its bound, given in the wire form.
			{Value: withContainer(map[string]any{
				"healthcheck": map[string]any{"cmd": []any{"pg_isready"}, "interval": "100ms"},
			}), Code: "GTE_VIOLATED", Path: "containers[0].healthcheck.interval"},
			// One case per cross-list rule; the error path is the rule id.
			{Value: dupNames, Code: "RULE_VIOLATED", Path: "machine-names-unique"},
			{Value: dupContainers, Code: "RULE_VIOLATED", Path: "container-names-unique"},
			{Value: badMachine, Code: "RULE_VIOLATED", Path: "container-machine-exists"},
			{Value: badFlowFrom, Code: "RULE_VIOLATED", Path: "flow-from-role-exists"},
			{Value: badFlowTo, Code: "RULE_VIOLATED", Path: "flow-to-role-exists"},
			{Value: badScrape, Code: "RULE_VIOLATED", Path: "scrape-role-exists"},
			{Value: badHostPrep, Code: "RULE_VIOLATED", Path: "host-prep-role-exists"},
			{Value: badRunner, Code: "RULE_VIOLATED", Path: "runner-role-exists"},
			{Value: badFlow, Code: "RULE_VIOLATED", Path: "flows[0]"},
		},
	})
}
