import { meMutations, meQueries } from '@api/queries/me'
import type { MePatch } from '@api/types'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useMe } from './useMe'

type Flags = NonNullable<MePatch['onboarding']>

/** The account's first-run progress and a setter that writes it to the server. */
export function useOnboarding() {
  const me = useMe()
  const qc = useQueryClient()
  const save = useMutation({
    mutationFn: (flags: Flags) => meMutations.onboarding(flags),
    onSuccess: (next) => qc.setQueryData(meQueries.me().queryKey, next),
  })
  return {
    welcomeDone: me.onboarding?.welcome_done ?? false,
    tourDone: me.onboarding?.tour_done ?? false,
    checklistHidden: me.onboarding?.checklist_hidden ?? false,
    set: (flags: Flags) => save.mutateAsync(flags),
  }
}
