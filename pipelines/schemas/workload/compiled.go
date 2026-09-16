package workload

import (
	"strings"

	schemapb "github.com/gopherex/schemapb/go/schemapb"
	"google.golang.org/protobuf/proto"
)

// NativeDriver reuses library driver and connection fields in Stroppy lowerCamel
// form. RunSpec must not become an unvalidated second entrance to the executor.
func NativeDriver() *schemapb.Schema {
	library := Stroppy()
	var out *schemapb.Schema
	for _, f := range library.GetFields() {
		if f.GetName() == "driver" {
			out = proto.CloneOf(f.GetObject().GetSchema())
		}
	}
	for _, f := range library.GetFields() {
		if f.GetName() != "connection" {
			continue
		}
		for _, auth := range f.GetOneOf().GetVariants()["ydb"].GetFields() {
			if auth.GetName() == "kind" || auth.GetName() == "ca_cert" {
				continue
			}
			out.Fields = append(out.Fields, proto.CloneOf(auth))
		}
	}
	nativeNames(out)
	out.Fields = append(out.Fields, schemapb.Str("caCertFile").Title("CA file").Group("Driver").Desc("Path inside the Stroppy container; use a shipped file or workload.ca_cert for inline PEM, not both.").MinLen(1).MaxLen(512).Done())
	return out
}

// Baseline reuses the machine self-check contract at the RunSpec entrance.
func Baseline() *schemapb.Schema {
	for _, f := range Stroppy().GetFields() {
		if f.GetName() == "baseline" {
			return proto.CloneOf(f.GetObject().GetSchema())
		}
	}
	panic("workload.stroppy has no baseline")
}

func nativeNames(s *schemapb.Schema) {
	for _, f := range s.GetFields() {
		parts := strings.Split(f.GetName(), "_")
		for i := 1; i < len(parts); i++ {
			if parts[i] != "" {
				parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
			}
		}
		f.Name = strings.Join(parts, "")
		if obj := f.GetObject(); obj != nil {
			nativeNames(obj.GetSchema())
		}
	}
}
