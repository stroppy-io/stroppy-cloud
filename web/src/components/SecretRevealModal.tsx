import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Alert, Button, ClipboardButton, Modal, Stack, Text, useStyles2 } from '@grafana/ui'
import { useTranslation } from 'react-i18next'

const getStyles = (theme: GrafanaTheme2) => ({
  secret: css({
    fontFamily: theme.typography.fontFamilyMonospace,
    fontSize: theme.typography.body.fontSize,
    padding: theme.spacing(1.5),
    background: theme.colors.background.secondary,
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    wordBreak: 'break-all',
    userSelect: 'all',
  }),
})

// One-time secret (API token, webhook signing key). The value is never shown again after close.
export function SecretRevealModal({
  isOpen,
  title,
  secret,
  description,
  onClose,
}: {
  isOpen: boolean
  title: string
  secret: string | undefined
  description?: string
  onClose: () => void
}) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  return (
    <Modal isOpen={isOpen} title={title} onDismiss={onClose} closeOnBackdropClick={false}>
      <Stack direction="column" gap={2}>
        <Alert severity="warning" title={t('settings.secret.onceTitle')}>
          {t('settings.secret.onceBody')}
        </Alert>
        {description && <Text color="secondary">{description}</Text>}
        <div className={styles.secret} data-testid="secret-value">
          {secret ?? ''}
        </div>
        <Modal.ButtonRow>
          <ClipboardButton getText={() => secret ?? ''} icon="copy" variant="primary">
            {t('common.actions.copy')}
          </ClipboardButton>
          <Button variant="secondary" onClick={onClose}>
            {t('settings.secret.done')}
          </Button>
        </Modal.ButtonRow>
      </Stack>
    </Modal>
  )
}
