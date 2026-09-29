import { type AdminTenant, adminMutations, adminQueries } from '@api/queries/admin'
import { toast } from '@app/Toaster'
import { Alert, Button, Combobox, Field, Modal, Stack, Text } from '@grafana/ui'
import { serverErrors } from '@helpers/form'
import { useInfiniteQuery, useMutation } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

export function AssignOwnerModal({
  tenant: tn,
  onClose,
  onSaved,
}: {
  tenant: AdminTenant
  onClose: () => void
  onSaved: () => Promise<void>
}) {
  const { t } = useTranslation()
  const users = useInfiniteQuery(adminQueries.users())
  const [userId, setUserId] = useState<string | null>(null)
  const [error, setError] = useState<string>()
  const assign = useMutation({
    mutationFn: (id: string) => adminMutations.assignOwner(tn.slug, id),
    onSuccess: async (next) => {
      toast.success(t('admin.tenants.ownerAssigned', { name: next.owner.display_name ?? '' }))
      await onSaved()
    },
    onError: (e) => {
      const s = serverErrors(e, t('common.errors.generic'))
      setError(s.fields.user_id ?? s.form)
    },
  })
  const options =
    users.data?.pages
      .flatMap((p) => p.data)
      .map((u) => ({
        label: u.display_name,
        value: u.id,
        description: `${u.email}${u.owned_tenant ? ` · ${t('admin.tenants.ownsAlready', { name: u.owned_tenant.name })}` : ''}`,
      })) ?? []
  return (
    <Modal isOpen title={t('admin.tenants.assignOwner')} onDismiss={onClose}>
      <Stack direction="column" gap={2}>
        <Text color="secondary">
          {t('admin.tenants.assignOwnerHint', {
            name: tn.name,
            owner: tn.owner.display_name ?? '—',
          })}
        </Text>
        {tn.status === 'orphaned' && (
          <Alert severity="warning" title={t('admin.tenants.orphanedTitle')}>
            {t('admin.tenants.orphanedBody')}
          </Alert>
        )}
        <Field label={t('admin.tenants.newOwner')} invalid={!!error} error={error}>
          <Combobox
            options={options}
            value={userId}
            isClearable
            loading={users.isPending}
            placeholder={t('admin.tenants.pickUser')}
            onChange={(o) => {
              setUserId(o?.value ?? null)
              setError(undefined)
            }}
          />
        </Field>
        <Modal.ButtonRow>
          <Button variant="secondary" onClick={onClose}>
            {t('common.actions.cancel')}
          </Button>
          <Button
            disabled={!userId || assign.isPending}
            onClick={() => userId && assign.mutate(userId)}
          >
            {t('admin.tenants.assignOwner')}
          </Button>
        </Modal.ButtonRow>
      </Stack>
    </Modal>
  )
}
