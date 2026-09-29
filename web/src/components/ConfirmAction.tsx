import { ConfirmModal } from '@grafana/ui'
import { type ReactNode, useState } from 'react'
import { useTranslation } from 'react-i18next'

// Wraps any trigger element with a confirmation modal. `confirmationText` forces typing (e.g. the name).
export function ConfirmAction({
  title,
  body,
  confirmText,
  confirmationText,
  destructive = true,
  onConfirm,
  children,
}: {
  title: string
  body: ReactNode
  confirmText?: string
  confirmationText?: string
  destructive?: boolean
  onConfirm: () => Promise<unknown> | undefined
  children: (open: () => void) => ReactNode
}) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [busy, setBusy] = useState(false)
  return (
    <>
      {children(() => setOpen(true))}
      <ConfirmModal
        isOpen={open}
        title={title}
        body={body}
        confirmText={
          confirmText ?? (destructive ? t('common.confirm.yesDelete') : t('common.actions.confirm'))
        }
        confirmButtonVariant={destructive ? 'destructive' : 'primary'}
        confirmationText={confirmationText}
        dismissText={t('common.actions.cancel')}
        disabled={busy}
        onConfirm={async () => {
          setBusy(true)
          try {
            await onConfirm()
            setOpen(false)
          } finally {
            setBusy(false)
          }
        }}
        onDismiss={() => setOpen(false)}
      />
    </>
  )
}
