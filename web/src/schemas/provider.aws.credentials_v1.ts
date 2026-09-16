// GENERATED from schemapb schema provider.aws.credentials@1 — do not edit.
// AWS IAM access key of the tenant account (write-only).

/** root */
export interface ProviderAwsCredentials1 {
  /** Access key id. IAM access key id; AKIA… for a long-term key, ASIA… for temporary STS credentials. */
  access_key_id: string;
  /** Secret access key. Secret half of the access key (40 base64 characters). */
  secret_access_key: string;
  /** Session token. STS session token; required with temporary (ASIA…) credentials, absent otherwise. */
  session_token?: string;
}
