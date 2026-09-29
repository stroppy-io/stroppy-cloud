package observe

import (
	"strconv"
	"testing"
	"time"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
)

func seqOf(origin time.Time, l LogLine) int {
	lines := []LogLine{l}
	AssignSeqs(origin, lines, true, nil)
	return lines[0].Seq
}

func TestSeqRoundTrip(t *testing.T) {
	created := time.Date(2026, 9, 29, 10, 0, 0, 123456789, time.UTC)
	origin := SeqOrigin(run.Run{CreatedAt: created})
	line := LogLine{Time: created.Add(90*24*time.Hour + 1234567), Message: "hello", Level: "info", Fields: map[string]string{"stream": "stdout", "graphene.agent": "a1"}}
	seq := seqOf(origin, line)
	if seq <= 0 || seq >= 1<<53 {
		t.Fatalf("seq %d is not an exact JSON number", seq)
	}
	a, ok := ParseSeq(origin, strconv.Itoa(seq))
	if !ok {
		t.Fatal("own seq did not parse")
	}
	if !a.At.Equal(line.Time.Truncate(time.Microsecond)) || a.Next().Sub(a.At) != time.Microsecond {
		t.Fatalf("anchor %v does not hold %v", a.At, line.Time)
	}
	if a.Bucket != LineHash(line) {
		t.Fatal("bucket lost")
	}
	// Same line, same seq: attribute order does not matter.
	again := line
	again.Fields = map[string]string{"graphene.agent": "a1", "stream": "stdout"}
	if seqOf(origin, again) != seq {
		t.Fatal("seq is not stable")
	}
	later := line
	later.Time = line.Time.Add(time.Microsecond)
	if seqOf(origin, later) <= seq {
		t.Fatal("seq does not follow time")
	}
}

func TestSeqBounds(t *testing.T) {
	origin := SeqOrigin(run.Run{CreatedAt: time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)})
	if seqOf(origin, LogLine{Time: origin.Add(-time.Second)}) != 0 {
		t.Fatal("a line before the origin has a seq")
	}
	if seqOf(origin, LogLine{Time: origin.Add(200 * 24 * time.Hour)}) != 0 {
		t.Fatal("a line beyond the seq range has a seq")
	}
	for _, c := range []string{"", "0", "-5", "12a", "eyJwIjoiIn0", "99999999999999999999"} {
		if _, ok := ParseSeq(origin, c); ok {
			t.Fatalf("%q parsed as a seq", c)
		}
	}
}

// collidingBodies are two bodies with one digest.
func collidingBodies(t *testing.T) (string, string) {
	t.Helper()
	seen := map[uint16]string{}
	for i := 0; ; i++ {
		body := "line-" + strconv.Itoa(i)
		h := LineHash(LogLine{Message: body})
		if other, ok := seen[h]; ok {
			return other, body
		}
		seen[h] = body
	}
}

func TestSeqCollisionsStayUnique(t *testing.T) {
	origin := SeqOrigin(run.Run{CreatedAt: time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)})
	at := origin.Add(48 * time.Hour)
	a, b := collidingBodies(t)
	// Store order within the microsecond; b's twin written twice.
	group := []LogLine{{Time: at, Message: a}, {Time: at.Add(100), Message: b}, {Time: at.Add(100), Message: b}}
	whole := append([]LogLine(nil), group...)
	AssignSeqs(origin, whole, true, nil)
	seen := map[int]bool{}
	for _, l := range whole {
		if l.Seq == 0 || seen[l.Seq] {
			t.Fatalf("seqs are not unique: %+v", whole)
		}
		seen[l.Seq] = true
	}
	// A page holding only part of the microsecond, newest first, gets the
	// same seqs once it is given the whole microsecond.
	part := []LogLine{group[2], group[1]}
	AssignSeqs(origin, part, false, map[int64][]LogLine{Micro(at): group})
	if part[0].Seq != whole[2].Seq || part[1].Seq != whole[1].Seq {
		t.Fatalf("page-dependent seqs: %d,%d vs %d,%d", part[0].Seq, part[1].Seq, whole[2].Seq, whole[1].Seq)
	}
	// Each seq resolves to its own bucket.
	buckets := SeqBuckets(group)
	for i, l := range whole {
		anchor, ok := ParseSeq(origin, strconv.Itoa(l.Seq))
		if !ok || anchor.Bucket != buckets[i] || Micro(anchor.At) != Micro(at) {
			t.Fatalf("seq %d resolves to %+v", l.Seq, anchor)
		}
	}
}
