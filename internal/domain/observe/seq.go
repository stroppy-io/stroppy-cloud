package observe

import (
	"hash/fnv"
	"maps"
	"slices"
	"strconv"
	"time"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
)

// A line's seq names its place in the store, so a link to a line can be
// followed back to it: the microsecond it was written at, counted from the
// run's origin, and in the low bits a bucket that tells apart the lines of
// one microsecond. The bucket is the line's digest (body, level,
// attributes); lines of one microsecond whose digests collide take the next
// free bucket in the store's order (time, stream, body). It fits a JSON
// number exactly (below 2^53) and does not depend on the page the line came
// with. It depends on the filter only when a collision's earlier line is
// filtered out — a link keeps its filter, so it still finds its line.
const (
	seqBucketBits = 10
	seqBucketMask = 1<<seqBucketBits - 1
	// seqMaxOffset keeps seq below 2^53: ~101 days of microseconds.
	seqMaxOffset = int64(1)<<(53-seqBucketBits) - 1
	// seqOriginLead puts the origin before anything the run could log,
	// clock skew of its machines included.
	seqOriginLead = 24 * time.Hour
)

// SeqOrigin is the instant a run's seqs count from.
func SeqOrigin(r run.Run) time.Time {
	return r.CreatedAt.UTC().Truncate(time.Second).Add(-seqOriginLead)
}

// LineAnchor is a seq resolved against the run's origin: the microsecond
// of the line and its bucket in it.
type LineAnchor struct {
	At     time.Time // the start of the line's microsecond
	Bucket uint16
}

// IsZero reports an unset anchor.
func (a LineAnchor) IsZero() bool { return a.At.IsZero() }

// Next is the end of the anchor's microsecond (exclusive).
func (a LineAnchor) Next() time.Time { return a.At.Add(time.Microsecond) }

// Micro is the microsecond a time falls in, the key of its group.
func Micro(t time.Time) int64 {
	ns := t.UnixNano()
	us := ns / 1000
	if ns < 0 && ns%1000 != 0 {
		us--
	}
	return us
}

// LineHash is the digest of a line: its body, level and attributes, cut
// to a bucket.
func LineHash(l LogLine) uint16 {
	h := fnv.New64a()
	write := func(s string) {
		_, _ = h.Write([]byte(s))
		_, _ = h.Write([]byte{0})
	}
	write(l.Message)
	write(l.Level)
	keys := make([]string, 0, len(l.Fields))
	for k := range l.Fields {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		write(k)
		write(l.Fields[k])
	}
	return uint16(h.Sum64() & seqBucketMask)
}

// SeqBuckets are the buckets of one microsecond's lines, given in the
// store's order: each takes its digest, or the next free bucket after it.
func SeqBuckets(group []LogLine) []uint16 {
	var used [seqBucketMask + 1]bool
	out := make([]uint16, len(group))
	for i, l := range group {
		b := LineHash(l)
		for n := 0; used[b] && n <= seqBucketMask; n++ {
			b = (b + 1) & seqBucketMask
		}
		used[b] = true
		out[i] = b
	}
	return out
}

// seqAt is the seq of a bucket in a line's microsecond, 0 when the time is
// outside what a seq can name (before the origin or ~100 days after it).
func seqAt(origin, t time.Time, bucket uint16) int {
	offset := Micro(t) - Micro(origin)
	if offset <= 0 || offset > seqMaxOffset {
		return 0
	}
	return int(offset<<seqBucketBits | int64(bucket))
}

// SameLine reports two reads of one record.
func SameLine(a, b LogLine) bool {
	return a.Time.Equal(b.Time) && a.Message == b.Message && a.Level == b.Level && maps.Equal(a.Fields, b.Fields)
}

// AssignSeqs sets the seq of every line. lines are one contiguous stretch
// of the selection, in the store's order (asc) or reversed. groups holds
// the whole microsecond, in the store's order, for microseconds the lines
// may hold only in part — the stretch's edges; a microsecond inside the
// stretch is whole on it.
func AssignSeqs(origin time.Time, lines []LogLine, asc bool, groups map[int64][]LogLine) {
	order := make([]int, len(lines))
	for i := range order {
		order[i] = i
	}
	if !asc {
		slices.Reverse(order)
	}
	for start := 0; start < len(order); {
		key := Micro(lines[order[start]].Time)
		end := start + 1
		for end < len(order) && Micro(lines[order[end]].Time) == key {
			end++
		}
		members := order[start:end]
		group, ok := groups[key]
		if !ok {
			group = make([]LogLine, len(members))
			for i, m := range members {
				group[i] = lines[m]
			}
		}
		buckets := SeqBuckets(group)
		next := 0
		for _, m := range members {
			bucket := LineHash(lines[m])
			for j := next; j < len(group); j++ {
				if SameLine(group[j], lines[m]) {
					bucket, next = buckets[j], j+1
					break
				}
			}
			lines[m].Seq = seqAt(origin, lines[m].Time, bucket)
		}
		start = end
	}
}

// ParseSeq reads a cursor that is a seq; ok is false for anything else
// (the opaque page cursors are never all digits).
func ParseSeq(origin time.Time, cursor string) (LineAnchor, bool) {
	if cursor == "" || len(cursor) > 16 {
		return LineAnchor{}, false
	}
	for _, c := range cursor {
		if c < '0' || c > '9' {
			return LineAnchor{}, false
		}
	}
	v, err := strconv.ParseInt(cursor, 10, 64)
	if err != nil || v <= 0 {
		return LineAnchor{}, false
	}
	offset := v >> seqBucketBits
	if offset <= 0 || offset > seqMaxOffset {
		return LineAnchor{}, false
	}
	at := time.UnixMicro(Micro(origin) + offset).UTC()
	return LineAnchor{At: at, Bucket: uint16(v & seqBucketMask)}, true
}
