import { keys } from '@api/queries/keys'
import { runMutations } from '@api/queries/runs'
import { IconButton } from '@grafana/ui'
import { useTenant } from '@hooks/useTenant'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

// Optimistic star toggle for any favoritable entity.
export function FavoriteButton({
  kind,
  id,
  value,
  size = 'md',
}: {
  kind: 'run' | 'database' | 'workload' | 'test' | 'suite' | 'schedule'
  id: string
  value: boolean | undefined
  size?: 'sm' | 'md' | 'lg'
}) {
  const { t } = useTranslation()
  const { slug } = useTenant()
  const qc = useQueryClient()
  const [on, setOn] = useState(!!value)
  const m = useMutation({
    mutationFn: (next: boolean) => runMutations.favorite(slug, kind, id, next),
    onMutate: (next) => setOn(next),
    onError: (_e, next) => setOn(!next),
    onSettled: () => void qc.invalidateQueries({ queryKey: keys.t(slug) }),
  })
  return (
    <IconButton
      name={on ? 'favorite' : 'star'}
      size={size}
      tooltip={on ? t('common.actions.unfavorite') : t('common.actions.favorite')}
      aria-pressed={on}
      onClick={() => m.mutate(!on)}
      data-no-row-click
    />
  )
}
