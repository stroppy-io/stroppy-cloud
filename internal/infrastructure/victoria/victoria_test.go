package victoria

import "testing"

func TestScopePromQL(t *testing.T) {
	const c, n = `r="x"`, `nr="x"`
	cases := map[string]string{
		`up`: `up{r="x" or nr="x"}`,
		`rate(node_cpu_seconds_total{mode="idle"}[5m])`: `rate(node_cpu_seconds_total{mode="idle",r="x" or mode="idle",nr="x"}[5m])`,
		`sum by (machine) (up) / count(up{a="b"})`:      `sum by (machine) (up{r="x" or nr="x"}) / count(up{a="b",r="x" or a="b",nr="x"})`,
		`avg_over_time(tps[1h30m]) > 100`:               `avg_over_time(tps{r="x" or nr="x"}[1h30m]) > 100`,
		`label_replace(up, "m", "$1", "x", "(.*)")`:     `label_replace(up{r="x" or nr="x"}, "m", "$1", "x", "(.*)")`,
		`{"graphene.entity"="docker/a"}`:                `{"graphene.entity"="docker/a",r="x" or "graphene.entity"="docker/a",nr="x"}`,
	}
	for in, want := range cases {
		if got := scopePromQL(in, c, n); got != want {
			t.Errorf("%s\n got %s\nwant %s", in, got, want)
		}
	}
}
