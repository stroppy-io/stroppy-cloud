import { getAccessToken } from '@lib/auth'
import { loadPublicConfig } from '@lib/config'
import { useQuery } from '@tanstack/react-query'

/**
 * Whether the visitor has a session — for public pages that greet a signed-in
 * visitor with «open the console» instead of «sign in». Never redirects.
 */
export function useSignedIn(): boolean {
  const q = useQuery({
    queryKey: ['auth', 'signed-in'],
    queryFn: async () => {
      await loadPublicConfig()
      return (await getAccessToken()) !== ''
    },
    staleTime: 60_000,
    retry: false,
  })
  return q.data ?? false
}
