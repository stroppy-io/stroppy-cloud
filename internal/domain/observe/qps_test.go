package observe

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
)

func TestSumSeries(t *testing.T) {
	got := sumSeries([]Series{
		{Points: [][2]float64{{20, 2}, {10, 1}}},
		{Points: [][2]float64{{10, 5}, {30, 7}}},
	})
	want := []run.Point{{T: 10000, V: 6}, {T: 20000, V: 2}, {T: 30000, V: 7}}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	if s := sumSeries(nil); s == nil || len(s) != 0 {
		t.Fatal("no series is an empty, non-nil answer")
	}
}
