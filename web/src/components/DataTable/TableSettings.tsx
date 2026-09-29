import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import {
  Button,
  Checkbox,
  RadioButtonGroup,
  Stack,
  Text,
  Toggletip,
  ToolbarButton,
  useStyles2,
} from '@grafana/ui'
import { useTranslation } from 'react-i18next'
import { columnTitle, DENSITIES, type PrefColumn, type TablePrefs } from './prefs'

const getStyles = (theme: GrafanaTheme2) => ({
  body: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(2), minWidth: 240 }),
  list: css({
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'flex-start',
    gap: theme.spacing(0.75),
    maxHeight: 320,
    overflowY: 'auto',
  }),
})

// «Вид таблицы»: row density + which columns are shown, stored per viewer (tables-guide §8).
// Locked columns (identity, actions) are listed checked and disabled so the list reads complete.
export function TableSettings<C extends PrefColumn>({ prefs }: { prefs: TablePrefs<C> }) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const options = prefs.all.filter((c) => columnTitle(c))
  return (
    <Toggletip
      placement="bottom-end"
      title={t('common.table.view')}
      closeButton
      content={
        <div className={styles.body}>
          <Stack direction="column" gap={1}>
            <Text variant="bodySmall" color="secondary">
              {t('common.table.density')}
            </Text>
            <RadioButtonGroup
              size="sm"
              fullWidth
              value={prefs.density}
              onChange={prefs.setDensity}
              options={DENSITIES.map((d) => ({
                value: d,
                label: t(`common.table.densities.${d}`),
              }))}
            />
          </Stack>
          <Stack direction="column" gap={1}>
            <Text variant="bodySmall" color="secondary">
              {t('common.table.columns')}
            </Text>
            <div className={styles.list}>
              {options.map((c) => (
                <Checkbox
                  key={c.id}
                  label={columnTitle(c)}
                  value={prefs.isVisible(c.id ?? '')}
                  disabled={c.hideable === false}
                  onChange={() => prefs.toggle(c.id ?? '')}
                />
              ))}
            </div>
          </Stack>
          <Button
            size="sm"
            variant="secondary"
            fill="outline"
            icon="history"
            disabled={!prefs.customized}
            onClick={prefs.reset}
          >
            {t('common.table.resetView')}
          </Button>
        </div>
      }
    >
      <ToolbarButton
        icon="columns"
        variant={prefs.customized ? 'active' : 'default'}
        tooltip={t('common.table.view')}
        aria-label={t('common.table.view')}
      />
    </Toggletip>
  )
}
