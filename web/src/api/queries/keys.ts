// Every query key starts with a scope prefix so switching tenant = disjoint cache.
export const keys = {
  me: () => ['me'] as const,
  admin: () => ['admin'] as const,
  catalog: () => ['catalog'] as const,
  public: () => ['public'] as const,
  t: (slug: string) => ['t', slug] as const,
}
