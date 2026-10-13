package compile_test

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/compile"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/dbkind"
)

// A kind the catalog offers as deployable must have a recipe here, or a
// test of it validates and then fails at launch.
func TestEveryDeployableKindHasRecipe(t *testing.T) {
	for _, d := range dbkind.Databases() {
		if d.Deployable && !compile.Supported(d.Kind) {
			t.Errorf("%s is deployable but has no recipe", d.Kind)
		}
	}
}
