// Package schematest is the shared harness every schema test runs through:
// build, compile, descriptor check, valid/invalid cases, template render and
// the protoJSON golden under testdata/<public id>.json.
package schematest

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	schemapb "github.com/gopherex/schemapb/go/schemapb"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/schemajson"
)

var update = flag.Bool("update", false, "rewrite golden files")

// Invalid is one value that must fail with Code at Path.
type Invalid struct {
	Value map[string]any
	Code  string // without the ERROR_CODE_ prefix, e.g. GTE_VIOLATED
	Path  string
}

// Cases drives Run.
type Cases struct {
	Valid    []map[string]any
	Invalid  []Invalid
	Render   string   // template name to render for every Valid value ("" = skip)
	Contains []string // substrings every rendered text must contain
	Formats  schemapb.FormatRegistry
}

// GoldenDir is where goldens live, relative to the schemas package root.
// Tests in sub-packages pass their own path via SetGoldenDir in TestMain, or
// rely on the default that walks up to a `testdata` directory.
var goldenDir string

// SetGoldenDir overrides the golden directory (absolute or relative to cwd).
func SetGoldenDir(dir string) { goldenDir = dir }

func findGoldenDir(t *testing.T) string {
	t.Helper()
	if goldenDir != "" {
		return goldenDir
	}
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		cand := filepath.Join(dir, "testdata")
		if st, err := os.Stat(cand); err == nil && st.IsDir() && filepath.Base(dir) == "schemas" {
			return cand
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("schematest: schemas/testdata not found above cwd")
	return ""
}

// Run executes the whole harness for one schema.
func Run(t *testing.T, s *schemapb.Schema, c Cases) {
	t.Helper()
	if s == nil {
		t.Fatal("nil schema")
	}
	if err := s.CheckDescriptor(); err != nil {
		t.Fatalf("descriptor: %v", err)
	}
	var opts []schemapb.CompileOption
	if c.Formats != nil {
		opts = append(opts, schemapb.WithFormats(c.Formats))
	}
	eng, err := schemapb.Compile(s, opts...)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	for i, v := range c.Valid {
		baked, res, err := eng.Bake(cloneMap(v))
		if err != nil {
			t.Fatalf("valid[%d]: bake error: %v", i, err)
		}
		if res.Blocking() {
			t.Fatalf("valid[%d]: unexpected errors:\n%s", i, format(res))
		}
		if c.Render != "" {
			text, err := baked.Render(schemapb.TemplateName(c.Render))
			if err != nil {
				t.Fatalf("valid[%d]: render %q: %v", i, c.Render, err)
			}
			if strings.TrimSpace(text) == "" {
				t.Fatalf("valid[%d]: render %q is empty", i, c.Render)
			}
			for _, sub := range c.Contains {
				if !strings.Contains(text, sub) {
					t.Errorf("valid[%d]: rendered %q lacks %q:\n%s", i, c.Render, sub, text)
				}
			}
		}
	}
	for i, inv := range c.Invalid {
		res := eng.Validate(cloneMap(inv.Value))
		found := false
		for _, e := range res.GetErrors() {
			code := strings.TrimPrefix(e.GetCode().String(), "ERROR_CODE_")
			if code == inv.Code && (inv.Path == "" || e.GetPath() == inv.Path) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("invalid[%d]: want %s at %q, got:\n%s", i, inv.Code, inv.Path, format(res))
		}
	}
	golden(t, s)
}

func golden(t *testing.T, s *schemapb.Schema) {
	t.Helper()
	dir := findGoldenDir(t)
	name := ids.Public(s.GetId()) + ".json"
	path := filepath.Join(dir, name)
	got, err := schemajson.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil { //nolint:gosec // golden file in the repo
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("golden %s missing (run with -update): %v", name, err)
	}
	var a, b schemapb.Schema
	if err := protojson.Unmarshal(want, &a); err != nil {
		t.Fatalf("golden %s unreadable: %v", name, err)
	}
	if err := protojson.Unmarshal(got, &b); err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(&a, &b) {
		t.Fatalf("golden %s differs from the built schema (run with -update after reviewing)", name)
	}
}

func format(res *schemapb.ValidationResult) string {
	var sb strings.Builder
	for _, e := range res.GetErrors() {
		sb.WriteString("  " + e.GetPath() + ": " + e.GetCode().String() + " " + e.GetMessage() + "\n")
	}
	return sb.String()
}

func cloneMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
