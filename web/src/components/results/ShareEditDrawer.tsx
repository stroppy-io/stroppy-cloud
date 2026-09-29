import { keys } from '@api/queries/keys'
import { shareMutations } from '@api/queries/results'
import type { Share } from '@api/types'
import { toast } from '@app/Toaster'
import { Button, Drawer, Field, Input, RadioButtonGroup, Select, Stack } from '@grafana/ui'
import { formatDateTime } from '@helpers/time'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

const TTL = ['never', '1d', '7d', '30d', '90d'] as const
const SCOPES: Share['scope'][] = ['overview', 'metrics', 'configs']

// Inline editor for a share: title, TTL (counted from now), scope.
export function ShareEditDrawer({
  slug,
  share,
  onClose,
}: {
  slug: string
  share: Share
  onClose: () => void
}) {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const [title, setTitle] = useState(share.title ?? '')
  const [ttl, setTtl] = useState<(typeof TTL)[number] | undefined>(undefined)
  const [scope, setScope] = useState<Share['scope']>(share.scope)
  const save = useMutation({
    mutationFn: () =>
      shareMutations.patch(slug, share.id, {
        title: title.trim() || null,
        scope,
        ...(ttl ? { ttl: ttl === 'never' ? null : ttl } : {}),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: [...keys.t(slug), 'shares'] })
      toast.success(t('results.shares.actions.saved'))
      onClose()
    },
    onError: (e: Error) => toast.error(e),
  })
  return (
    <Drawer title={t('results.shares.edit.title')} subtitle={share.url} size="sm" onClose={onClose}>
      <Stack direction="column" gap={2}>
        <Field label={t('results.shares.edit.titleField')} htmlFor="share-title">
          <Input
            id="share-title"
            value={title}
            placeholder={t('results.shares.edit.titlePlaceholder')}
            onChange={(e) => setTitle(e.currentTarget.value)}
          />
        </Field>
        <Field
          label={t('results.shares.edit.ttl')}
          description={t('results.shares.edit.ttlHint')}
          htmlFor="share-ttl"
        >
          <Select
            inputId="share-ttl"
            options={TTL.map((v) => ({
              label: t(`results.shares.edit.ttlOptions.${v}`),
              value: v,
            }))}
            value={ttl ?? null}
            placeholder={
              share.expires_at
                ? formatDateTime(share.expires_at)
                : t('results.shares.edit.ttlOptions.never')
            }
            onChange={(o) => setTtl(o?.value)}
            isClearable
          />
        </Field>
        <Field
          label={t('results.shares.edit.scope')}
          description={t(`results.shares.scopeHint.${scope}`)}
        >
          <RadioButtonGroup
            fullWidth
            options={SCOPES.map((s) => ({ label: t(`results.shares.scope.${s}`), value: s }))}
            value={scope}
            onChange={setScope}
          />
        </Field>
        <Stack justifyContent="flex-end" gap={1}>
          <Button variant="secondary" onClick={onClose} disabled={save.isPending}>
            {t('common.actions.cancel')}
          </Button>
          <Button onClick={() => save.mutate()} disabled={save.isPending}>
            {t('common.actions.save')}
          </Button>
        </Stack>
      </Stack>
    </Drawer>
  )
}
