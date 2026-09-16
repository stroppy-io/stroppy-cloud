// GENERATED from schemapb schema provider.registry.credentials@1 — do not edit.
// Docker registry login for private database images (write-only).

/** root */
export interface ProviderRegistryCredentials1 {
  /** Registry. Registry host as it appears in the image reference, e.g. registry.stroppy.io. */
  registry: string;
  /** Username. Registry user; for a token-only registry use the vendor's placeholder user. */
  username: string;
  /** Password / token. Registry password or access token. */
  password: string;
}
