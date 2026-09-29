import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Modal, Stack, Text, useStyles2 } from '@grafana/ui'
import { useTranslation } from 'react-i18next'
import { HOTKEYS } from './hotkeys'

const getStyles = (theme: GrafanaTheme2) => ({
  grid: css({
    display: 'grid',
    gridTemplateColumns: '1fr 1fr',
    gap: theme.spacing(3),
    [theme.breakpoints.down('sm')]: { gridTemplateColumns: '1fr' },
  }),
  row: css({
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    gap: theme.spacing(2),
    padding: theme.spacing(0.75, 0),
    borderBottom: `1px solid ${theme.colors.border.weak}`,
  }),
})

// Key hint rendered as subtle secondary text (no hand-drawn key caps).
export function Kbd({ keys }: { keys: string }) {
  return (
    <Text variant="bodySmall" color="secondary" tabular>
      {keys.replace('Escape', 'Esc')}
    </Text>
  )
}

export function ShortcutsModal({ isOpen, onClose }: { isOpen: boolean; onClose: () => void }) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const groups: { id: 'goto' | 'global'; title: string }[] = [
    { id: 'goto', title: t('nav.shortcutGoto') },
    { id: 'global', title: t('nav.shortcutGlobal') },
  ]
  return (
    <Modal title={t('nav.shortcutsTitle')} isOpen={isOpen} onDismiss={onClose}>
      <div className={styles.grid}>
        {groups.map((g) => (
          <Stack key={g.id} direction="column" gap={0.5}>
            <Text weight="medium">{g.title}</Text>
            {HOTKEYS.filter((h) => h.group === g.id).map((h) => (
              <div key={h.keys} className={styles.row}>
                <span>{t(`nav.${h.labelKey}`)}</span>
                <Kbd keys={h.keys} />
              </div>
            ))}
          </Stack>
        ))}
      </div>
    </Modal>
  )
}
