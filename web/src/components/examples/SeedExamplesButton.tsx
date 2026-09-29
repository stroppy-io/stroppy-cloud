import { keys } from '@api/queries/keys'
import { exampleMutations, exampleQueries } from '@api/queries/results'
import type { Example } from '@api/types'
import { toast } from '@app/Toaster'
import { Button } from '@grafana/ui'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

// Empty-library seeder: clones every gallery example of one kind into the tenant, named by the
// example title. Offered only in an empty list, so names cannot collide with the tenant's own.
export function SeedExamplesButton({
  slug,
  kind,
  disabled,
}: {
  slug: string
  kind: Exclude<Example['kind'], 'suite'>
  disabled?: boolean
}) {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const list = useQuery(exampleQueries.list({ kind }))
  const examples = list.data?.data ?? []
  const seed = useMutation({
    mutationFn: async () => {
      let created = 0
      for (const e of examples) {
        await exampleMutations.clone(slug, e.id, e.title)
        created++
      }
      return created
    },
    onSuccess: (count) => toast.success(t('examples.seed.done', { count })),
    onError: (e: Error) => toast.error(e, { title: t('examples.seed.failed') }),
    onSettled: () => void qc.invalidateQueries({ queryKey: keys.t(slug) }),
  })
  if (list.isSuccess && examples.length === 0) return null
  return (
    <Button
      variant="secondary"
      icon={seed.isPending ? 'spinner' : 'book-open'}
      disabled={disabled || !list.isSuccess || seed.isPending}
      onClick={() => seed.mutate()}
    >
      {t('examples.seed.button', { count: examples.length })}
    </Button>
  )
}
