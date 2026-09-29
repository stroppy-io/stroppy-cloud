import { type LibraryKind, libraryByKind } from '@api/queries/library'
import { ErrorState } from '@app/ErrorState'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Combobox, Field, LoadingPlaceholder, Stack, Text, useStyles2 } from '@grafana/ui'
import { useTenant } from '@hooks/useTenant'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

const getStyles = (theme: GrafanaTheme2) => ({
  table: css({
    width: '100%',
    borderCollapse: 'collapse',
    fontSize: theme.typography.bodySmall.fontSize,
    'th, td': {
      padding: theme.spacing(0.75, 1),
      borderBottom: `1px solid ${theme.colors.border.weak}`,
      textAlign: 'left',
      verticalAlign: 'top',
    },
    th: { color: theme.colors.text.secondary, fontWeight: theme.typography.fontWeightMedium },
  }),
  path: css({ fontFamily: theme.typography.fontFamilyMonospace, whiteSpace: 'nowrap' }),
  val: css({
    fontFamily: theme.typography.fontFamilyMonospace,
    whiteSpace: 'pre-wrap',
    overflowWrap: 'anywhere',
    maxWidth: 360,
  }),
  add: css({ color: theme.colors.success.text }),
  remove: css({ color: theme.colors.error.text }),
  replace: css({ color: theme.colors.warning.text }),
  op: css({
    display: 'inline-block',
    minWidth: 18,
    textAlign: 'center',
    fontFamily: theme.typography.fontFamilyMonospace,
    fontWeight: theme.typography.fontWeightBold,
  }),
})

const fmt = (v: unknown) => (v === undefined ? '' : typeof v === 'string' ? v : JSON.stringify(v))

// Field-by-field diff of the current record vs another one of the same kind (`:diff?a&b`).
export function DiffPanel({
  kind,
  currentId,
  currentName,
  otherId,
  onOtherChange,
}: {
  kind: LibraryKind
  currentId: string
  currentName: string
  otherId: string | undefined
  onOtherChange: (id: string | undefined) => void
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const { slug } = useTenant()
  const options = useQuery(libraryByKind[kind].options(slug))
  const diff = useQuery(libraryByKind[kind].diff(slug, currentId, otherId ?? ''))
  const items = (options.data?.data ?? []).filter((e) => e.id !== currentId)
  return (
    <Stack direction="column" gap={2}>
      <Field label={t('library.diff.compareWith')} htmlFor="diff-other">
        <Combobox
          id="diff-other"
          width={40}
          isClearable
          placeholder={t('library.diff.pick')}
          loading={options.isPending}
          options={items.map((e) => ({ label: e.name, value: e.id, description: e.description }))}
          value={otherId ?? null}
          onChange={(o) => onOtherChange(o?.value)}
        />
      </Field>
      {!otherId ? (
        <Text color="secondary">{t('library.diff.hint')}</Text>
      ) : diff.isPending ? (
        <LoadingPlaceholder text={t('common.misc.loading')} />
      ) : diff.isError ? (
        <ErrorState compact error={diff.error} onRetry={() => void diff.refetch()} />
      ) : !diff.data.changes.length ? (
        <Text color="secondary">{t('library.diff.identical')}</Text>
      ) : (
        <table className={styles.table}>
          <thead>
            <tr>
              <th />
              <th>{t('library.diff.path')}</th>
              <th>{currentName}</th>
              <th>{items.find((e) => e.id === otherId)?.name ?? otherId}</th>
            </tr>
          </thead>
          <tbody>
            {diff.data.changes.map((c) => (
              <tr key={`${c.op}-${c.path}`}>
                <td>
                  <span className={`${styles.op} ${styles[c.op]}`}>
                    {c.op === 'add' ? '+' : c.op === 'remove' ? '−' : '~'}
                  </span>
                </td>
                <td className={styles.path}>{c.path}</td>
                <td className={`${styles.val} ${c.op !== 'add' ? styles.remove : ''}`}>
                  {fmt(c.a)}
                </td>
                <td className={`${styles.val} ${c.op !== 'remove' ? styles.add : ''}`}>
                  {fmt(c.b)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </Stack>
  )
}
