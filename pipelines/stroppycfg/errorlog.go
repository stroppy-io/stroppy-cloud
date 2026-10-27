package stroppycfg

import (
	"bufio"
	"encoding/json"
	"io"
	"math"
	"regexp"
	"sort"
	"time"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// errorLogLimit is how many kinds of error a segment keeps, the most
// frequent first; the rest only count in Stroppy's totals.
const errorLogLimit = 20

// logLine is the part of a Stroppy (zap JSON) log line the error log reads.
type logLine struct {
	Level      string  `json:"level"`
	TS         float64 `json:"ts"`
	Msg        string  `json:"msg"`
	Err        string  `json:"err"`
	Error      string  `json:"error"`
	ErrorClass string  `json:"error_class"`
	SQL        string  `json:"sql"`
}

// ErrorLog groups the error and warning lines of a Stroppy log by level,
// message, error text and class: a reader of the report sees which errors
// a count is made of without downloading the log. Lines that are not JSON
// (the summary, the banner) are skipped.
func ErrorLog(r io.Reader) []spec.ErrorLogEntry {
	type key struct{ level, msg, err, class string }
	groups := map[key]*spec.ErrorLogEntry{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 16<<20)
	for sc.Scan() {
		b := sc.Bytes()
		if len(b) == 0 || b[0] != '{' {
			continue
		}
		var l logLine
		if json.Unmarshal(b, &l) != nil {
			continue
		}
		switch l.Level {
		case "error", "warn", "dpanic", "panic", "fatal":
		default:
			continue
		}
		text := l.Err
		if text == "" {
			text = l.Error
		}
		k := key{l.Level, cut(scrub(l.Msg), 256), cut(scrub(text), 512), l.ErrorClass}
		g, ok := groups[k]
		if !ok {
			g = &spec.ErrorLogEntry{Level: k.level, Message: k.msg, Error: k.err, Class: k.class, SQL: cut(scrub(l.SQL), 512)}
			groups[k] = g
		}
		g.Count++
		if at := tsOf(l.TS); !at.IsZero() {
			if g.FirstAt.IsZero() || at.Before(g.FirstAt) {
				g.FirstAt = at
			}
			if at.After(g.LastAt) {
				g.LastAt = at
			}
		}
	}
	out := make([]spec.ErrorLogEntry, 0, len(groups))
	for _, g := range groups {
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].FirstAt.Before(out[j].FirstAt)
	})
	if len(out) > errorLogLimit {
		out = out[:errorLogLimit]
	}
	return out
}

// tsOf is zap's epoch-seconds timestamp.
func tsOf(ts float64) time.Time {
	if ts <= 0 {
		return time.Time{}
	}
	sec, frac := math.Modf(ts)
	return time.Unix(int64(sec), int64(frac*1e9)).UTC()
}

// The error log reaches shared reports, and a driver error can carry the
// connection string: a URL's password and key=value password pairs are
// masked.
var (
	urlPassword = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://[^:/@\s]*):[^@\s]*@`)
	kvPassword  = regexp.MustCompile(`(?i)\b(password|passwd|pwd|token|secret)\s*=\s*('[^']*'|"[^"]*"|[^\s&;]+)`)
)

func scrub(s string) string {
	s = urlPassword.ReplaceAllString(s, "$1:***@")
	return kvPassword.ReplaceAllString(s, "$1=***")
}

func cut(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
