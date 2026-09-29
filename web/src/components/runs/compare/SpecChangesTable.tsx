import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Badge, Text, useStyles2 } from '@grafana/ui'
import { formatDiffValue, type SpecChange } from '@helpers/spec-diff'
import { useTranslation } from 'react-i18next'

const getStyles = (theme: GrafanaTheme2) => ({
  table: css({
    width: '100%',
    borderCollapse: 'collapse',
    fontSize: theme.typography.bodySmall.fontSize,
    th: {
      textAlign: 'left',
      padding: theme.spacing(0.75, 1),
      color: theme.colors.text.secondary,
      borderBottom: `1px solid ${theme.colors.border.weak}`,
      fontWeight: theme.typography.fontWeightMedium,
    },
    td: {
      padding: theme.spacing(0.5, 1),
      borderBottom: `1px solid ${theme.colors.border.weak}`,
      verticalAlign: 'top',
      fontFamily: theme.typography.fontFamilyMonospace,
      overflowWrap: 'anywhere',
    },
  }),
  path: css({ color: theme.colors.text.primary, whiteSpace: 'nowrap' }),
  a: css({ color: theme.colors.error.text, background: theme.colors.error.transparent }),
  b: css({ color: theme.colors.success.text, background: theme.colors.success.transparent }),
  dim: css({ color: theme.colors.text.disabled }),
})

// path / op / a → b rows. Used by the compare page and the spec-vs-test drawer.
export function SpecChangesTable({
  changes,
  aLabel,
  bLabel,
}: {
  changes: SpecChange[]
  aLabel: string
  bLabel: string
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  if (!changes.length)
    return (
      <Text color="secondary" variant="bodySmall">
        {t('runs.compare.noSpecDiff')}
      </Text>
    )
  return (
    <table className={styles.table}>
      <thead>
        <tr>
          <th>{t('runs.compare.path')}</th>
          <th style={{ width: 90 }}>{t('runs.compare.op')}</th>
          <th>{aLabel}</th>
          <th>{bLabel}</th>
        </tr>
      </thead>
      <tbody>
        {changes.map((c) => (
          <tr key={`${c.path}-${c.op}`}>
            <td className={styles.path}>{c.path}</td>
            <td>
              <Badge
                text={t(`runs.compare.ops.${c.op}`)}
                color={c.op === 'add' ? 'green' : c.op === 'remove' ? 'red' : 'blue'}
              />
            </td>
            <td className={cx(c.op !== 'add' && styles.a, c.op === 'add' && styles.dim)}>
              {formatDiffValue(c.a)}
            </td>
            <td className={cx(c.op !== 'remove' && styles.b, c.op === 'remove' && styles.dim)}>
              {formatDiffValue(c.b)}
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}
