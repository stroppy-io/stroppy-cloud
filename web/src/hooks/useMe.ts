import { meQueries } from '@api/queries/me'
import { useSuspenseQuery } from '@tanstack/react-query'

export function useMe() {
  return useSuspenseQuery(meQueries.me()).data
}
