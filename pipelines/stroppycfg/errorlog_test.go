package stroppycfg

import (
	"strings"
	"testing"
)

func TestErrorLogGroupsStroppyErrors(t *testing.T) {
	log := strings.Join([]string{
		`{"level":"info","ts":1793053687.6,"msg":"Loaded config file"}`,
		`{"level":"error","ts":1793053700.5,"msg":"Query","err":"ERROR: could not serialize access due to concurrent update (SQLSTATE 40001)","sql":"UPDATE warehouse SET w_ytd = w_ytd + $1"}`,
		`{"level":"error","ts":1793053701.5,"msg":"Query","err":"ERROR: could not serialize access due to concurrent update (SQLSTATE 40001)","sql":"UPDATE district SET d_ytd = d_ytd + $1"}`,
		`{"level":"error","ts":1793053702,"msg":"Query","err":"ERROR: could not serialize access due to concurrent delete (SQLSTATE 40001)"}`,
		`{"level":"warn","ts":1793053703,"msg":"nonfatal errors continue","error_class":"serialization"}`,
		`Error: not json`,
	}, "\n")
	got := ErrorLog(strings.NewReader(log))
	if len(got) != 3 {
		t.Fatalf("%d groups: %+v", len(got), got)
	}
	top := got[0]
	if top.Count != 2 || top.Level != "error" || !strings.Contains(top.Error, "concurrent update") || top.SQL == "" {
		t.Fatalf("top group: %+v", top)
	}
	if top.FirstAt.Unix() != 1793053700 || top.LastAt.Unix() != 1793053701 {
		t.Fatalf("times: %v %v", top.FirstAt, top.LastAt)
	}
	if got[2].Class != "serialization" || got[2].Level != "warn" {
		t.Fatalf("warn group: %+v", got[2])
	}
}

func TestErrorLogMasksCredentials(t *testing.T) {
	log := `{"level":"error","ts":1,"msg":"connect","err":"failed to connect to postgresql://postgres:stroppy_postgres@10.0.0.1:5432/db: host=10.0.0.1 user=postgres password=hunter2 token='abc def'"}`
	got := ErrorLog(strings.NewReader(log))[0].Error
	for _, secret := range []string{"stroppy_postgres", "hunter2", "abc def"} {
		if strings.Contains(got, secret) {
			t.Fatalf("%q leaks %q", got, secret)
		}
	}
	if !strings.Contains(got, "postgresql://postgres:***@10.0.0.1") || !strings.Contains(got, "user=postgres") {
		t.Fatalf("masked too much: %q", got)
	}
}
