// GENERATED from schemapb schema spec.provider_config@1 — do not edit.
// Configure or delete a persistent cloud profile through the Graphene installation.

/** root */
export interface SpecProviderConfig1 {
  /** Action. Ensure validates credentials and applies configuration; delete waits for cleanup. */
  action: "ensure" | "delete";
  /** Provider. Cloud the credentials belong to. */
  provider: "yandex" | "aws";
  /** Profile ID. Canonical UUID of the provider profile; combined with the worker namespace for isolation. */
  profile_id: string;
  /** Credentials reference. Graphene secret name provider-<profile_id> with optional immutable version suffix; no credential contents. */
  credentials_secret: string;
  /** Settings. Baked provider settings, required for ensure. */
  settings?: unknown;
}
