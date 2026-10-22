package activities

import (
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNetProbeClassifiesOutcomes(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()
	closed, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closedAddr := closed.Addr().String()
	_ = closed.Close()

	start := time.Now()
	res, err := NetProbe(t.Context(), NetProbeRequest{
		Targets: []NetTarget{
			{Machine: "open", Address: ln.Addr().String()},
			{Machine: "closed", Address: closedAddr},
			// TEST-NET-1 (RFC 5737): routed nowhere, the SYN gets no answer.
			{Machine: "blackhole", Address: "192.0.2.1:5432"},
		},
		DialTimeout: 300 * time.Millisecond, Window: 1500 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took > 5*time.Second {
		t.Fatalf("probe not bounded: %s", took)
	}
	if len(res.Pairs) != 3 {
		t.Fatalf("pairs %+v", res.Pairs)
	}
	if p := res.Pairs[0]; p.Outcome != NetOK || p.Attempts != 1 || p.Error != "" {
		t.Fatalf("open: %+v", p)
	}
	if p := res.Pairs[1]; p.Outcome != NetRefused || p.Attempts < 2 || !strings.Contains(p.Error, "refused") {
		t.Fatalf("closed port is retried within the window and refused: %+v", p)
	}
	switch p := res.Pairs[2]; p.Outcome {
	case NetTimeout:
	case NetOther:
		// A sandbox without a default route answers ENETUNREACH at once.
		t.Logf("blackhole answered at once (no route here): %s", p.Error)
	default:
		t.Fatalf("blackhole: %+v", p)
	}
}

func TestNetProbeBoundsTheRequest(t *testing.T) {
	if _, err := NetProbe(t.Context(), NetProbeRequest{Targets: make([]NetTarget, probeMaxTargets+1)}); err == nil {
		t.Fatal("an unbounded request must be refused")
	}
}

func TestClassifyDialText(t *testing.T) {
	for in, want := range map[string]NetOutcome{
		"": NetOK,
		"context deadline exceeded (last dial error: dial tcp 10.0.0.3:5432: connect: connection refused)": NetRefused,
		"context deadline exceeded (last dial error: dial tcp 10.0.0.3:5432: i/o timeout)":                 NetTimeout,
		"dial tcp 10.0.0.3:5432: connect: no route to host":                                                NetOther,
	} {
		if got := ClassifyDialText(in); got != want {
			t.Errorf("%q: %s, want %s", in, got, want)
		}
	}
}

func TestNetSnapshotWritesBoundedReport(t *testing.T) {
	t.Setenv("GRAPHENE_WORKSPACE", t.TempDir())
	res, err := NetSnapshot(t.Context(), NetSnapshotRequest{Label: "probe-db-1", Machine: "db-1", Context: []string{"db-2:5432 from db-1: timeout"}})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(res.Path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, want := range []string{"# network snapshot of db-1", "db-2:5432 from db-1: timeout", "$ ip route", "$ ss -ltnp", "$ cat /etc/sysctl.d/90-stroppy-*.conf"} {
		if !strings.Contains(text, want) {
			t.Errorf("snapshot lacks %q", want)
		}
	}
	if res.Commands != len(snapshotCommands) {
		t.Fatalf("%+v", res)
	}
	if _, err := NetSnapshot(t.Context(), NetSnapshotRequest{Label: "../x"}); err == nil {
		t.Fatal("a label must not escape the snapshot directory")
	}
}

func TestCappedKeepsTheHead(t *testing.T) {
	c := &capped{max: 4}
	_, _ = c.Write([]byte("abcdef"))
	_, _ = c.Write([]byte("gh"))
	if got := c.String(); !strings.HasPrefix(got, "abcd") || !strings.Contains(got, "truncated: 4 more bytes") {
		t.Fatalf("%q", got)
	}
}
