import type { Schemas } from '@api/types'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Text, useStyles2 } from '@grafana/ui'
import { requirementReason } from '@helpers/topology'
import { useTranslation } from 'react-i18next'
import { roleRank } from './SizesEditor'

type Requirements = Schemas['Requirements']
type RoleSizes = Schemas['RoleSizes']
type SizeSpec = Schemas['SizeSpec']

const getStyles = (theme: GrafanaTheme2) => ({
  table: css({
    width: '100%',
    borderCollapse: 'collapse',
    fontSize: theme.typography.bodySmall.fontSize,
    'th, td': {
      padding: theme.spacing(0.75, 1),
      borderBottom: `1px solid ${theme.colors.border.weak}`,
      textAlign: 'left',
      whiteSpace: 'nowrap',
    },
    th: { color: theme.colors.text.secondary, fontWeight: theme.typography.fontWeightMedium },
    'tr:last-child td': { borderBottom: 0 },
    'td.num': { textAlign: 'right', fontFamily: theme.typography.fontFamilyMonospace },
  }),
  ok: css({ color: theme.colors.success.text }),
  bad: css({ color: theme.colors.error.text }),
  // Why the role needs this much — under the role, so the table stays narrow.
  reason: css({
    display: 'block',
    color: theme.colors.text.secondary,
    whiteSpace: 'normal',
    fontWeight: theme.typography.fontWeightRegular,
  }),
  empty: css({ color: theme.colors.text.secondary, padding: theme.spacing(1) }),
})

// Minimal hardware per role; when `sizes`+`sizeTable` are given, shows the chosen size and whether it fits.
export function RequirementsTable({
  requirements,
  sizes,
  sizeTable,
}: {
  requirements: Requirements | undefined
  sizes?: RoleSizes
  sizeTable?: SizeSpec[]
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const rows = Object.entries(requirements ?? {}).sort(
    ([a], [b]) => roleRank(a) - roleRank(b) || a.localeCompare(b)
  )
  if (!rows.length) return <div className={styles.empty}>{t('library.requirements.empty')}</div>
  const withSizes = !!sizes && !!sizeTable
  return (
    <table className={styles.table}>
      <thead>
        <tr>
          <th>{t('library.requirements.role')}</th>
          <th style={{ textAlign: 'right' }}>{t('library.requirements.cpu')}</th>
          <th style={{ textAlign: 'right' }}>{t('library.requirements.memory')}</th>
          <th style={{ textAlign: 'right' }}>{t('library.requirements.disk')}</th>
          {withSizes && <th>{t('library.requirements.chosen')}</th>}
        </tr>
      </thead>
      <tbody>
        {rows.map(([role, r]) => {
          const chosen = sizes?.[role]
          const spec = sizeTable?.find((s) => s.size === chosen?.size)
          const fits = spec && spec.cpu >= (r.cpu ?? 0) && spec.memory_gb >= (r.memory_gb ?? 0)
          return (
            <tr key={role}>
              <td style={{ whiteSpace: 'normal' }}>
                <Text weight="medium">{role}</Text>
                {r.reason && <span className={styles.reason}>{requirementReason(r)}</span>}
              </td>
              <td className="num">{r.cpu ?? '—'}</td>
              <td className="num">{r.memory_gb !== undefined ? `${r.memory_gb} GB` : '—'}</td>
              <td className="num">{r.disk_gb !== undefined ? `${r.disk_gb} GB` : '—'}</td>
              {withSizes && (
                <td>
                  {chosen ? (
                    <span
                      className={fits ? styles.ok : styles.bad}
                      title={spec ? `${spec.cpu} vCPU · ${spec.memory_gb} GB` : undefined}
                    >
                      {chosen.size}
                    </span>
                  ) : (
                    <span className={styles.bad}>{t('library.requirements.notSet')}</span>
                  )}
                </td>
              )}
            </tr>
          )
        })}
      </tbody>
    </table>
  )
}
