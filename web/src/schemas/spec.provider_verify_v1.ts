// GENERATED from schemapb schema spec.provider_verify@1 — do not edit.
// Params of stroppy-provider-verify: can these credentials create and destroy resources?

/** root */
export interface SpecProviderVerify1 {
  /** Provider. Cloud the credentials belong to. */
  provider: "yandex" | "aws";
  /** Settings. Baked provider.<kind>.settings value being verified. */
  settings: unknown;
  /** Credentials secret. Name of the Graphene secret holding the credentials — never the value. */
  credentials_secret: string;
  /** Dry run. Only read the account (identity, permissions); do not create a probe resource. */
  dry_run?: boolean;
}
