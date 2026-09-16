// GENERATED from schemapb schema spec.result.provider_verify@1 — do not edit.
// Result of stroppy-provider-verify: whether the profile is usable, and why not.

/** object  */
export interface SpecResultProviderVerify1Item {
  /** Permission. Provider permission or IAM action that was probed. */
  name: string;
  /** Granted. */
  granted: boolean;
}

/** root */
export interface SpecResultProviderVerify1 {
  /** Usable. True when the credentials authenticate and carry the permissions a run needs. */
  ok: boolean;
  /** Account. Service account id (yandex) or IAM ARN / account id (aws) the credentials resolve to. */
  account_id?: string;
  /** Scope. Folder id (yandex) or region/project the check ran against. */
  scope?: string;
  /** Permissions. Per-permission verdict; a missing one is why ok is false. */
  permissions?: Array<SpecResultProviderVerify1Item>;
  /** Error. Provider error text when ok is false; shown as the profile status reason. */
  error?: string;
}
