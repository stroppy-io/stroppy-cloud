// Package uitext reads the SPA's locale files, so tests next to a server
// vocabulary (roles, event kinds, metric groups) can check the UI has a
// text for every value the server sends: a value without one reaches the
// user as a raw key, an English server title or a catch-all "Other".
package uitext

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Langs are the SPA's languages.
var Langs = []string{"ru", "en"}

// root is the repository root, found from this file.
func root() string {
	_, file, _, _ := runtime.Caller(0) //nolint:dogsled // only the file
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// Has reports whether `ns.path.to.key` has a non-empty string (or a
// nested object) in the language's locale.
func Has(lang, key string) (bool, error) {
	ns, rest, _ := strings.Cut(key, ".")
	raw, err := os.ReadFile(filepath.Join(root(), "web", "src", "locales", lang, ns+".json"))
	if err != nil {
		return false, err
	}
	var node any
	if err := json.Unmarshal(raw, &node); err != nil {
		return false, err
	}
	for _, part := range strings.Split(rest, ".") {
		m, ok := node.(map[string]any)
		if !ok {
			return false, nil
		}
		if node, ok = m[part]; !ok {
			return false, nil
		}
	}
	switch v := node.(type) {
	case string:
		return v != "", nil
	case map[string]any:
		return len(v) > 0, nil
	}
	return false, nil
}
