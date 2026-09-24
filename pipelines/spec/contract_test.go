package spec

import (
	"reflect"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas"
)

// Check BOTH sets, recursively. A new executable struct field without a schema
// and a schema field discarded by Go decoding must fail the same contract test.
// JSON/map payloads intentionally delegate their shape to their owning schema.
func TestPipelineFieldContract(t *testing.T) {
	for _, tc := range []struct {
		id    string
		value any
	}{
		{"spec.run@1", Run{}},
		{"test.runtime@1", Runtime{}},
		{"spec.suite@1", Suite{}},
		{"spec.result.run@1", Result{}},
		{"spec.provider_verify@1", ProviderVerify{}},
		{"spec.provider_config@1", ProviderConfig{}},
		{"spec.result.provider_verify@1", ProviderVerifyResult{}},
		{"spec.quotas@1", Quotas{}},
		{"spec.result.quotas@1", QuotasResult{}},
		{"workload.segment@1", Segment{}},
	} {
		t.Run(tc.id, func(t *testing.T) {
			s := schemas.ByID()[tc.id]
			checkFields(t, tc.id, s, reflect.TypeOf(tc.value), s.GetDefs())
		})
	}
}

func checkFields(t *testing.T, path string, schema *schemapb.Schema, typ reflect.Type, defs map[string]*schemapb.Schema) {
	t.Helper()
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct || typ == reflect.TypeFor[WorkloadParams]() {
		return
	}
	fields := map[string]reflect.Type{}
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		name := strings.Split(f.Tag.Get("json"), ",")[0]
		if name != "" && name != "-" {
			fields[name] = f.Type
		}
	}
	for _, f := range schema.GetFields() {
		name := f.GetName()
		ft, ok := fields[name]
		if !ok {
			t.Errorf("%s.%s is accepted by schema but discarded by Go", path, name)
			continue
		}
		delete(fields, name)
		checkField(t, path+"."+name, f, ft, defs)
	}
	for name := range fields {
		t.Errorf("%s.%s exists in Go but cannot be expressed by schema", path, name)
	}
}

func checkField(t *testing.T, path string, f *schemapb.Schema_Field, typ reflect.Type, defs map[string]*schemapb.Schema) {
	t.Helper()
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if ref := f.GetRef(); ref != nil {
		target, ok := defs[ref.GetName()]
		if !ok {
			t.Fatalf("%s: unresolved schema ref %s", path, ref.GetName())
		}
		checkFields(t, path, target, typ, defs)
	}
	if union := f.GetOneOf(); union != nil {
		for name, variant := range union.GetVariants() {
			combined := proto.Clone(variant).(*schemapb.Schema)
			hasDiscriminator := false
			for _, field := range combined.Fields {
				if field.GetName() == union.GetDiscriminator() {
					hasDiscriminator = true
				}
			}
			if !hasDiscriminator {
				combined.Fields = append([]*schemapb.Schema_Field{schemapb.Str(schemapb.FieldName(union.GetDiscriminator())).Done()}, variant.GetFields()...)
			}
			checkFields(t, path+"["+name+"]", combined, typ, defs)
		}
	}
	if f.GetObject() != nil {
		checkFields(t, path, f.GetObject().GetSchema(), typ, defs)
	}
	if f.GetList() != nil && typ.Kind() == reflect.Slice {
		for _, item := range f.GetList().GetItems() {
			checkField(t, path+"[]", item, typ.Elem(), defs)
		}
	}
}
