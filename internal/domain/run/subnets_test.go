package run

import (
	"encoding/json"
	"errors"
	"net/netip"
	"testing"

	pipelinespec "github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

func netRun(holds bool, cidrs ...string) NetworkRun {
	var r pipelinespec.Run
	for i, c := range cidrs {
		r.Network.Subnets = append(r.Network.Subnets, pipelinespec.Subnet{Zone: string(rune('a' + i)), CIDR: c})
	}
	raw, _ := json.Marshal(r) //nolint:errcheck // a plain struct
	return NetworkRun{Spec: raw, Holds: holds}
}

func cidrs(subnets []pipelinespec.Subnet) []string {
	out := make([]string, 0, len(subnets))
	for _, s := range subnets {
		out = append(out, s.CIDR)
	}
	return out
}

func equal(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestSubnetSizing(t *testing.T) {
	pool := newSubnetPool(nil)
	// One zone per entry, each sized to its hosts with headroom: /28 holds
	// up to 4 hosts, /27 up to 12, /26 up to 28, /24 at most.
	got, err := pool.allocate("10.128.0.0/16", []pipelinespec.ZoneDemand{
		{Zone: "ru-central1-a", Hosts: 3}, {Zone: "ru-central1-b", Hosts: 9}, {Zone: "ru-central1-d", Hosts: 20}, {Zone: "ru-central1-e", Hosts: 200},
	})
	if err != nil {
		t.Fatal(err)
	}
	equal(t, cidrs(got), []string{"10.128.0.0/28", "10.128.0.32/27", "10.128.0.64/26", "10.128.1.0/24"})
	for i, s := range got {
		if s.Zone != []string{"ru-central1-a", "ru-central1-b", "ru-central1-d", "ru-central1-e"}[i] {
			t.Fatalf("zone order changed: %+v", got)
		}
	}
	if _, err := pool.allocate("10.128.0.0/16", []pipelinespec.ZoneDemand{{Zone: "a", Hosts: 300}}); err == nil || errors.Is(err, errPoolFull) {
		t.Fatalf("a zone larger than a /24 never fits, got %v", err)
	}
}

func TestSubnetRoundRobinAndHeld(t *testing.T) {
	one := []pipelinespec.ZoneDemand{{Zone: "d", Hosts: 2}}
	// Held: a live run, a kept stand and a recently finished run (Holds);
	// the latest allocation finished long ago — its block is free, but the
	// cursor still starts after it.
	pool := newSubnetPool([]NetworkRun{
		netRun(true, "10.0.0.0/28"),
		netRun(true, "10.0.0.16/28"),
		netRun(true, "10.0.0.48/28"),
		netRun(false, "10.0.0.64/28"),
	})
	var got []string
	for {
		s, err := pool.allocate("10.0.0.0/25", one)
		if errors.Is(err, errPoolFull) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, cidrs(s)...)
	}
	// From the cursor (10.0.0.80) to the pool end, wrap, first fit skips
	// the held blocks, and the block the finished run had comes last.
	equal(t, got, []string{"10.0.0.80/28", "10.0.0.96/28", "10.0.0.112/28", "10.0.0.32/28", "10.0.0.64/28"})
}

func TestSubnetCursorWrapsAndExhausts(t *testing.T) {
	one := []pipelinespec.ZoneDemand{{Zone: "d", Hosts: 1}}
	pool := newSubnetPool([]NetworkRun{netRun(false, "10.0.0.32/28")})
	var got []string
	for {
		s, err := pool.allocate("10.0.0.0/26", one)
		if errors.Is(err, errPoolFull) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, cidrs(s)...)
	}
	// After the latest allocation (…32/28) first, then wrap to the start;
	// the block that run had is reused last.
	equal(t, got, []string{"10.0.0.48/28", "10.0.0.0/28", "10.0.0.16/28", "10.0.0.32/28"})
}

func TestSubnetAlignmentAndAllOrNothing(t *testing.T) {
	// The cursor sits mid-way; a /27 aligns up past it.
	pool := newSubnetPool([]NetworkRun{netRun(true, "10.0.0.0/28")})
	got, err := pool.allocate("10.0.0.0/25", []pipelinespec.ZoneDemand{{Zone: "a", Hosts: 10}})
	if err != nil {
		t.Fatal(err)
	}
	equal(t, cidrs(got), []string{"10.0.0.32/27"})
	// Two zones, room for one: the run takes nothing.
	before := len(pool.held)
	cursor := pool.cursor
	_, err = pool.allocate("10.0.0.0/25", []pipelinespec.ZoneDemand{{Zone: "a", Hosts: 25}, {Zone: "b", Hosts: 25}})
	if !errors.Is(err, errPoolFull) {
		t.Fatalf("want a full pool, got %v", err)
	}
	if len(pool.held) != before || pool.cursor != cursor {
		t.Fatal("a run that does not fit changed the pool")
	}
	// Every block handed out is inside the pool and disjoint.
	all := append([]netip.Prefix(nil), pool.held...)
	for i, a := range all {
		if !netip.MustParsePrefix("10.0.0.0/25").Contains(a.Addr()) {
			t.Fatalf("%s outside the pool", a)
		}
		for _, b := range all[i+1:] {
			if a.Overlaps(b) {
				t.Fatalf("%s overlaps %s", a, b)
			}
		}
	}
}

func TestSubnetBadPool(t *testing.T) {
	for _, pool := range []string{"::/64", "nonsense", "0.0.0.0/0"} {
		if _, err := newSubnetPool(nil).allocate(pool, []pipelinespec.ZoneDemand{{Zone: "a"}}); err == nil || errors.Is(err, errPoolFull) {
			t.Fatalf("%s: got %v", pool, err)
		}
	}
	// A pool smaller than the block waits rather than overflowing it.
	if _, err := newSubnetPool(nil).allocate("10.0.0.0/28", []pipelinespec.ZoneDemand{{Zone: "a", Hosts: 10}}); !errors.Is(err, errPoolFull) {
		t.Fatalf("got %v", err)
	}
}
