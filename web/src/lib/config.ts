import { api, unwrap } from '@api/client'
import type { PublicConfig } from '@api/types'
import { setKratosBase } from '@lib/auth'

let cached: PublicConfig | undefined

export async function loadPublicConfig(): Promise<PublicConfig> {
  if (cached) return cached
  cached = await unwrap(api.GET('/api/v1/public/config'))
  // The SPA talks to Kratos directly (API flows); the origin is only known
  // once the server answered.
  setKratosBase(cached.auth.kratos.public_url)
  return cached
}

export function getPublicConfig(): PublicConfig | undefined {
  return cached
}
