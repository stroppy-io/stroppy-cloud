import { type AdminTenant, adminMutations } from '@api/queries/admin'
import { toast } from '@app/Toaster'
import { Alert, Button, Field, Modal, Stack, TextArea } from '@grafana/ui'
import { serverErrors } from '@helpers/form'
import { useMutation } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

export function SuspendModal({
  tenant: tn,
  onClose,
  onSaved,
}: {
  tenant: AdminTenant
  onClose: () => void
  onSaved: () => Promise<void>
}) {
  const { t } = useTranslation()
  const [reason, setReason] = useState('')
  const [error, setError] = useState<string>()
  const suspend = useMutation({
    mutationFn: () => adminMutations.suspendTenant(tn.slug, reason.trim() || undefined),
    onSuccess: async () => {
      toast.success(t('admin.tenants.suspended', { name: tn.name }))
      await onSaved()
    },
    onError: (e) => setError(serverErrors(e, t('common.errors.generic')).form),
  })
  const live = tn.counters?.runs_running ?? 0
  return (
    <Modal isOpen title={t('admin.tenants.suspendTitle', { name: tn.name })} onDismiss={onClose}>
      <Stack direction="column" gap={2}>
        <Alert severity="warning" title={t('admin.tenants.suspendWarnTitle')}>
          {t('admin.tenants.suspendWarnBody')}
          {live > 0 ? ` ${t('admin.tenants.suspendLive', { count: live })}` : ''}
        </Alert>
        <Field
          label={t('admin.tenants.suspendReason')}
          description={t('admin.tenants.suspendReasonHint')}
        >
          <TextArea rows={3} value={reason} onChange={(e) => setReason(e.currentTarget.value)} />
        </Field>
        {error && <Alert severity="error" title={error} />}
        <Modal.ButtonRow>
          <Button variant="secondary" onClick={onClose}>
            {t('common.actions.cancel')}
          </Button>
          <Button
            variant="destructive"
            icon="pause"
            disabled={suspend.isPending}
            onClick={() => suspend.mutate()}
          >
            {t('admin.tenants.suspend')}
          </Button>
        </Modal.ButtonRow>
      </Stack>
    </Modal>
  )
}
