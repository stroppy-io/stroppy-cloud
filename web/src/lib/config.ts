import { api, unwrap } from '@api/client'
import type { PublicConfig } from '@api/types'

let cached: PublicConfig | undefined

export async function loadPublicConfig(): Promise<PublicConfig> {
  if (cached) return cached
  cached = await unwrap(api.GET('/api/v1/public/config'))
  return cached
}

export function getPublicConfig(): PublicConfig | undefined {
  return cached
}
