package system

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
	specschema "github.com/stroppy-io/stroppy-cloud/pipelines/schemas/spec"
)

// Runtime exposes advanced deployment controls without duplicating RunSpec fields.
// Selectors are checked after recipe compilation; unknown selectors are errors.
func Runtime() *schemapb.Schema {
	run := specschema.Run()
	// The kind is what the container IS — decided by the recipe that made
	// it, like its name and its machine; an override does not rename it.
	container := patchSchema(runItem("containers"), "name", "role", "machine", "kind")
	s := schemapb.NewSchema(ids.Test("runtime", 1)).Strict().Coerce().
		DefSchema("machine", Machine()).DefSchema("container", container).
		Fields(
			schemapb.MapOf("machines", schemapb.Ref("value", "machine")).Title("Machine overrides").Group("Machines").Desc("Exact generated machine name to hardware overrides, applied after role presets.").MaxEntries(64),
			schemapb.List("containers", schemapb.Object("",
				schemapb.Str("role").Title("Role").Group("Selector").Desc("Select containers by role, optionally narrowed by name.").Pattern(`^[a-z][a-z0-9_-]{0,63}$`),
				schemapb.Str("name").Title("Name").Group("Selector").Desc("Exact generated container name; at least one selector is required.").Pattern(`^[a-z][a-z0-9_-]{0,63}$`),
				schemapb.Ref("set", "container").Title("Settings").Group("Deployment").Desc("Explicit fields replace defaults; env/ulimits merge by key and files merge by absolute path. Empty files list removes generated files.").Required(),
			).Strict().Rule(schemapb.Rule(`"role" in this || "name" in this`, "at least one container selector is required").ID("selector-required"))).Title("Container overrides").Group("Deployment").Desc("Applied in order; later matching entries win. Unknown selectors fail compilation.").MaxItems(256),
		).MustBuild()
	for _, f := range run.Fields {
		switch f.Name {
		case "containers", "host_prep", "scrapes", "flows", "network", "observability", "result_expectations", "workload":
			f = proto.CloneOf(f)
			if f.Name == "containers" {
				f.Name = "additional_containers"
			}
			f.Required = false
			if f.Name == "workload" {
				f.GetObject().Schema = patchSchema(f.GetObject().GetSchema())
			}
			stripDefaults(f.ProtoReflect())
			s.Fields = append(s.Fields, f)
		}
	}
	for name, def := range run.Defs {
		s.Defs[name] = proto.CloneOf(def)
	}
	return s
}

func runItem(name string) *schemapb.Schema {
	for _, f := range specschema.Run().Fields {
		if f.Name == name {
			return proto.CloneOf(f.GetList().GetItems()[0].GetObject().GetSchema())
		}
	}
	panic("unknown RunSpec field: " + name)
}

func patchSchema(s *schemapb.Schema, excluded ...string) *schemapb.Schema {
	fields := s.Fields[:0]
	for _, f := range s.Fields {
		skip := false
		for _, key := range excluded {
			if f.Name == key {
				skip = true
			}
		}
		if skip {
			continue
		}
		f.Required = false
		if f.Name == "healthcheck" || f.Name == "boot_disk" {
			f.Nullable = true
		}
		stripDefaults(f.ProtoReflect())
		fields = append(fields, f)
	}
	s.Fields = fields
	return s
}

// An absent patch field MUST remain absent; native defaults belong to the base.
func stripDefaults(m protoreflect.Message) {
	m.Range(func(f protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		if string(f.Name()) == "default" {
			m.Clear(f)
			return true
		}
		switch {
		case f.IsList() && f.Message() != nil:
			l := v.List()
			for i := 0; i < l.Len(); i++ {
				stripDefaults(l.Get(i).Message())
			}
		case f.IsMap() && f.MapValue().Message() != nil:
			v.Map().Range(func(_ protoreflect.MapKey, x protoreflect.Value) bool { stripDefaults(x.Message()); return true })
		case f.Message() != nil && !f.IsMap():
			stripDefaults(v.Message())
		}
		return true
	})
}
