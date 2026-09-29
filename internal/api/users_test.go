package api

import (
	"reflect"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/oas"
)

func TestWalkUserRefsReachesNestedRefs(t *testing.T) {
	list := &oas.ListRunsOK{Data: []oas.Run{{Author: oas.UserRef{ID: "a"}}, {Author: oas.UserRef{ID: "b"}}}}
	payload := map[string]any{"data": []any{&oas.Run{Author: oas.UserRef{ID: "c"}}}, "n": 1}
	for _, v := range []any{list, payload, oas.Share{CreatedBy: oas.NewOptUserRef(oas.UserRef{ID: "d"})}} {
		root := reflect.New(reflect.TypeOf(v)).Elem()
		root.Set(reflect.ValueOf(v))
		walkUserRefs(root, func(r *oas.UserRef) { r.DisplayName = oas.NewOptString("n-" + r.ID) })
		v = root.Interface()
		var got []string
		walkUserRefs(root, func(r *oas.UserRef) { got = append(got, r.DisplayName.Or("")) })
		if len(got) == 0 {
			t.Fatalf("%T: no refs visited", v)
		}
		for _, g := range got {
			if g == "" {
				t.Fatalf("%T: ref not filled: %v", v, got)
			}
		}
	}
	if list.Data[1].Author.DisplayName.Or("") != "n-b" {
		t.Fatal("pointer root not filled in place")
	}
}

func TestMayHoldUserRefPrunes(t *testing.T) {
	if mayHoldUserRef(reflect.TypeFor[oas.PageMeta]()) {
		t.Fatal("PageMeta holds no UserRef")
	}
	if !mayHoldUserRef(reflect.TypeFor[*oas.ListRunsOK]()) {
		t.Fatal("RunList holds UserRefs")
	}
}
