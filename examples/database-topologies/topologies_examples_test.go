package database_topologies

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stroppy-io/hatchet-workflow/internal/core/protoyaml"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/topology"
	databasepb "github.com/stroppy-io/hatchet-workflow/internal/proto/database"
)

func TestExampleTopologies_ParseAndValidate(t *testing.T) {
	dir := filepath.Join(".")

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %s: %v", dir, err)
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		files = append(files, filepath.Join(dir, entry.Name()))
	}

	sort.Strings(files)
	if len(files) == 0 {
		t.Fatalf("no topology yaml files found in %s", dir)
	}

	for _, file := range files {
		file := file
		t.Run(filepath.Base(file), func(t *testing.T) {
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("read file %s: %v", file, err)
			}

			var tmpl databasepb.Database_Template
			if err := protoyaml.UnmarshalStrict(raw, &tmpl); err != nil {
				t.Fatalf("unmarshal yaml %s: %v", file, err)
			}

			if err := topology.ValidateDatabaseTemplate(context.Background(), &tmpl); err != nil {
				t.Fatalf("validate template %s: %v", file, err)
			}
		})
	}
}
