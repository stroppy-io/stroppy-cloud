// GENERATED from schemapb schema spec.quotas@1 — do not edit.
// Params of stroppy-quotas: read the provider limits and usage of one profile.

/** root */
export interface SpecQuotas1 {
  /** Provider. Cloud the credentials belong to. */
  provider: "yandex" | "aws";
  /** Settings. Baked provider.<kind>.settings value; picks the folder or account to read. */
  settings: unknown;
  /** Credentials secret. Name of the Graphene secret holding the credentials — never the value. */
  credentials_secret: string;
  /** Location. Zone (yandex) or region (aws) to read zone-scoped quotas for; empty reads all. */
  location?: string;
}
