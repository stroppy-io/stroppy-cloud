import { isApiError } from '@api/errors'
import { keys } from '@api/queries/keys'
import { type LibraryKind, libraryByKind } from '@api/queries/library'
import type { Entity, Schemas } from '@api/types'

import { toast } from '@app/Toaster'
import { Button, ConfirmModal, Dropdown, IconButton, Menu, Stack, Text } from '@grafana/ui'
import { toYaml } from '@helpers/yaml'
import { useTenant } from '@hooks/useTenant'
import { downloadText, safeFilename } from '@lib/download'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { UsagesList } from './UsagesList'

type Usage = Schemas['Usage']

const DETAIL_ROUTE = {
  database: '/t/$slug/library/databases/$id',
  workload: '/t/$slug/library/workloads/$id',
  test: '/t/$slug/library/tests/$id',
} as const

// Clone / export (JSON, YAML) / delete for any library record. Delete is usage-guarded: the server's
// 409 lists dependents; databases/workloads can be inlined into them instead.
export function EntityActionsMenu({
  kind,
  entity,
  usages,
  asButton,
  onDeleted,
}: {
  kind: LibraryKind
  entity: Entity
  usages?: Usage[]
  asButton?: boolean
  onDeleted?: () => void
}) {
  const { t } = useTranslation()
  const { slug, can } = useTenant()
  const qc = useQueryClient()
  const navigate = useNavigate()
  const [confirm, setConfirm] = useState<'delete' | 'inline' | null>(null)
  const canEdit = can('edit-library')
  const invalidate = () => qc.invalidateQueries({ queryKey: keys.t(slug) })

  const clone = useMutation({
    mutationFn: () => libraryByKind[kind].clone(slug, entity.id) as Promise<Entity>,
    onSuccess: (created) => {
      void invalidate()
      toast.success(t('library.actions.cloned', { name: created.name }))
      void navigate({ to: DETAIL_ROUTE[kind], params: { slug, id: created.id } })
    },
    onError: (e) => toast.error(e),
  })
  const exportDoc = useMutation({
    mutationFn: async (format: 'json' | 'yaml') => {
      const doc = await qc.fetchQuery(libraryByKind[kind].export(slug, entity.id))
      const name = safeFilename(entity.name)
      if (format === 'json') downloadText(`${name}.json`, JSON.stringify(doc, null, 2))
      else downloadText(`${name}.yaml`, `${toYaml(doc)}\n`, 'application/yaml')
    },
    onError: (e) => toast.error(e),
  })
  const remove = useMutation({
    mutationFn: (inline: boolean) =>
      kind === 'test'
        ? libraryByKind.test.remove(slug, entity.id)
        : libraryByKind[kind].remove(slug, entity.id, inline),
    onSuccess: () => {
      void invalidate()
      toast.success(t('library.actions.deleted', { name: entity.name }))
      setConfirm(null)
      onDeleted?.()
    },
    onError: (e) => {
      if (isApiError(e) && e.code === 'entity_in_use') {
        toast.error(e)
        setConfirm(null)
      } else toast.error(e)
    },
  })

  const inUse = (usages?.length ?? 0) > 0
  const menu = (
    <Menu>
      <Menu.Item
        icon="copy"
        label={t('common.actions.clone')}
        disabled={!canEdit}
        onClick={() => clone.mutate()}
      />
      <Menu.Item
        icon="download-alt"
        label={t('library.actions.exportJson')}
        onClick={() => exportDoc.mutate('json')}
      />
      <Menu.Item
        icon="file-alt"
        label={t('library.actions.exportYaml')}
        onClick={() => exportDoc.mutate('yaml')}
      />
      <Menu.Divider />
      <Menu.Item
        icon="trash-alt"
        label={t('common.actions.delete')}
        destructive
        disabled={!canEdit}
        onClick={() => setConfirm('delete')}
      />
    </Menu>
  )
  return (
    <>
      <Dropdown overlay={menu} placement="bottom-end">
        {asButton ? (
          <Button variant="secondary" icon="ellipsis-v" aria-label={t('common.actions.more')}>
            {t('common.actions.more')}
          </Button>
        ) : (
          <IconButton name="ellipsis-v" tooltip={t('common.actions.more')} data-no-row-click />
        )}
      </Dropdown>
      <ConfirmModal
        isOpen={confirm !== null}
        title={t('common.confirm.deleteTitle', { name: entity.name })}
        body={
          inUse ? (
            <Stack direction="column" gap={1}>
              <Text>
                {t('library.actions.inUse', {
                  count: usages?.length ?? 0,
                  kind: t(`library.kind.${kind}`),
                })}
              </Text>
              <UsagesList usages={usages} />
              {kind !== 'test' && <Text color="secondary">{t('library.actions.inlineHint')}</Text>}
            </Stack>
          ) : (
            t('common.confirm.deleteBody')
          )
        }
        confirmText={
          inUse && kind !== 'test'
            ? t('library.actions.deleteAndInline')
            : t('common.confirm.yesDelete')
        }
        confirmButtonVariant="destructive"
        confirmationText={inUse ? entity.name : undefined}
        dismissText={t('common.actions.cancel')}
        disabled={remove.isPending || (inUse && kind === 'test')}
        onConfirm={() => remove.mutate(inUse && kind !== 'test')}
        onDismiss={() => setConfirm(null)}
      />
    </>
  )
}
