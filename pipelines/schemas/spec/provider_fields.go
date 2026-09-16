package spec

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"
	"google.golang.org/protobuf/proto"
)

func providerFields(settings schemapb.DefName) []schemapb.FieldDef {
	return []schemapb.FieldDef{
		schemapb.Ref("settings", settings).Title("Settings").Group("Provider").Desc("Baked provider placement settings.").Required(),
		schemapb.Str("credentials_secret").Title("Credentials secret").Group("Provider").Desc("Graphene secret name, never a credential value.").Pattern(secretNamePattern).Required(),
		schemapb.Str("provider_config_name").Title("ProviderConfig").Group("Provider").Desc("Crossplane ProviderConfig used by the run.").Pattern(namePattern).Required(),
		schemapb.Str("registry_secret").Title("Registry secret").Group("Provider").Desc("Optional Graphene secret with registry credentials.").Pattern(secretNamePattern),
	}
}

// A compiled RunSpec carries the resolved network separately. Old live inputs
// omit the profile's network selector; omission means the run-owned network.
// Keep all explicit settings validated by the original provider contract.
func runProviderSettings(schema *schemapb.Schema) *schemapb.Schema {
	out := proto.CloneOf(schema)
	for _, f := range out.Fields {
		if f.GetName() == "network" {
			f.Required = false
		}
	}
	return out
}
