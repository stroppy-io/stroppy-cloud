package errs

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Every reason the server can send has its text in both SPA locales
// (`common.errors.reasons.<reason>`): a reason without one reaches the user
// as the English detail.
func TestReasonsHaveUIText(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	because := regexp.MustCompile(`\.Because\("([a-z0-9_]+)"`)
	notFound := regexp.MustCompile(`errs\.NotFound\("([^"]+)"\)`)
	reasons := map[string]string{}
	// NotFound(what) with a variable `what` (library.owned) names these kinds.
	for _, kind := range []string{"database", "workload", "test"} {
		reasons[kind+"_not_found"] = "internal/domain/library/service.go"
	}
	err := filepath.WalkDir(filepath.Join(root, "internal"), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "oas" {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for _, m := range because.FindAllSubmatch(src, -1) {
			reasons[string(m[1])] = p
		}
		for _, m := range notFound.FindAllSubmatch(src, -1) {
			reasons[reasonKey(string(m[1]))+"_not_found"] = p
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(reasons) < 50 {
		t.Fatalf("found only %d reasons; the scan is broken", len(reasons))
	}
	for _, lang := range []string{"ru", "en"} {
		raw, err := os.ReadFile(filepath.Join(root, "web", "src", "locales", lang, "common.json"))
		if err != nil {
			t.Fatal(err)
		}
		var locale struct {
			Errors struct {
				Reasons map[string]string `json:"reasons"`
			} `json:"errors"`
		}
		if err := json.Unmarshal(raw, &locale); err != nil {
			t.Fatal(err)
		}
		for reason, file := range reasons {
			if locale.Errors.Reasons[reason] == "" {
				t.Errorf("%s: reason %q (%s) has no text in common.errors.reasons", lang, reason, file)
			}
		}
	}
}
