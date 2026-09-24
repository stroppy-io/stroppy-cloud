package spec

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

func ProviderConfig() *schemapb.Schema {
	return schemapb.NewSchema(ids.Spec("provider_config", 1)).Descr("Configure or delete a persistent cloud profile through the Graphene installation.").Strict().Coerce().Fields(
		schemapb.Choice("action").Title("Action").Group("Lifecycle").Desc("Ensure validates credentials and applies configuration; delete waits for cleanup.").Opt(schemapb.StrV("ensure"), "Configure").Opt(schemapb.StrV("delete"), "Delete").Required(),
		providerKind(),
		schemapb.Str("profile_id").Title("Profile ID").Group("Provider").Desc("Canonical UUID of the provider profile; combined with the worker namespace for isolation.").Pattern("^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$").Required(),
		schemapb.Str("credentials_secret").Title("Credentials reference").Group("Provider").Desc("Graphene secret name provider-<profile_id> with optional immutable version suffix; no credential contents.").Pattern(secretNamePattern).Required(),
		schemapb.JSON("settings").Title("Settings").Group("Provider").Desc("Baked provider settings, required for ensure."),
	).Rules(schemapb.Rule("root.credentials_secret == 'provider-' + root.profile_id || root.credentials_secret.startsWith('provider-' + root.profile_id + '-')", "Credential reference must belong to this profile").ID("profile-credentials")).MustBuild()
}
