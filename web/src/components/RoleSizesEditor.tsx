import type { Schemas } from '@api/types'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Combobox, IconButton, useStyles2 } from '@grafana/ui'
import { SIZE_ORDER, sortRoles } from '@helpers/sizes'
import { useTranslation } from 'react-i18next'

type RoleSizes = Schemas['RoleSizes']

const getStyles = (theme: GrafanaTheme2) => ({
  row: css({ display: 'flex', flexWrap: 'wrap', gap: theme.spacing(1), alignItems: 'center' }),
  role: css({
    display: 'inline-flex',
    alignItems: 'center',
    gap: theme.spacing(0.5),
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.secondary,
    paddingLeft: theme.spacing(1),
  }),
  label: css({
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    fontFamily: theme.typography.fontFamilyMonospace,
    minWidth: 40,
  }),
})

// Compact per-role size picker: `db [M ▾]  runner [S ▾]`. Roles come from the caller (union of
// the tests' roles); the value keeps disk settings of existing entries untouched.
export function RoleSizesEditor({
  value,
  onChange,
  roles,
  disabled,
  onRemove,
}: {
  value: RoleSizes | undefined
  onChange: (next: RoleSizes) => void
  roles?: string[]
  disabled?: boolean
  onRemove?: () => void
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const allRoles = sortRoles([...new Set([...(roles ?? []), ...Object.keys(value ?? {})])])
  const options = SIZE_ORDER.map((s) => ({ label: s, value: s }))
  return (
    <div className={styles.row}>
      {allRoles.map((role) => (
        <span key={role} className={styles.role}>
          <span className={styles.label}>{role}</span>
          <Combobox<Schemas['Size']>
            width={9}
            options={options}
            value={value?.[role]?.size ?? null}
            disabled={disabled}
            aria-label={role}
            onChange={(o) =>
              onChange({ ...(value ?? {}), [role]: { ...value?.[role], size: o.value } })
            }
          />
        </span>
      ))}
      {onRemove && (
        <IconButton
          name="trash-alt"
          tooltip={t('common.actions.remove')}
          onClick={onRemove}
          disabled={disabled}
        />
      )}
    </div>
  )
}
