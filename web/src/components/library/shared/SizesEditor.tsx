import type { Schemas } from '@api/types'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Button, Combobox, Icon, Input, Stack, Text, Tooltip, useStyles2 } from '@grafana/ui'
import { roleLabel } from '@helpers/roles'
import { useTranslation } from 'react-i18next'
import { suggestedRoleSize, suggestedText } from './suggested'

type RoleSizes = Schemas['RoleSizes']
type Requirements = Schemas['Requirements']
type SizeSpec = Schemas['SizeSpec']
type Size = Schemas['Size']
type Issue = NonNullable<Schemas['Fit']['issues']>[number]

// Families in reading order; a family's own role first, then its kin
// (db, db-replica, …).
const ORDER = ['db', 'proxy', 'etcd', 'coordinator', 'runner']
export const roleRank = (role: string) => {
  const family = role.split('-')[0]
  const i = ORDER.indexOf(family)
  return (i === -1 ? 99 : i) * 10 + (role === family ? 0 : 1)
}

const getStyles = (theme: GrafanaTheme2) => ({
  grid: css({
    display: 'grid',
    gridTemplateColumns: 'minmax(130px, 170px) minmax(160px, 1fr) auto',
    gap: theme.spacing(1, 2),
    alignItems: 'start',
  }),
  head: css({
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
  }),
  role: css({
    paddingTop: theme.spacing(0.75),
    fontWeight: theme.typography.fontWeightMedium,
    minWidth: 0,
  }),
  req: css({
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    fontFamily: theme.typography.fontFamilyMonospace,
    fontWeight: theme.typography.fontWeightRegular,
    whiteSpace: 'nowrap',
  }),
  cell: css({ minWidth: 0 }),
  issue: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(0.5),
    fontSize: theme.typography.bodySmall.fontSize,
    marginTop: theme.spacing(0.5),
  }),
  error: css({ color: theme.colors.error.text }),
  warning: css({ color: theme.colors.warning.text }),
  ok: css({ color: theme.colors.success.text }),
  auto: css({
    marginLeft: theme.spacing(0.75),
    padding: theme.spacing(0, 0.5),
    borderRadius: theme.shape.radius.default,
    border: `1px solid ${theme.colors.info.border}`,
    color: theme.colors.info.text,
    fontSize: theme.typography.bodySmall.fontSize,
    fontWeight: theme.typography.fontWeightRegular,
  }),
})

// Size per role with the requirement next to it, fits/suggested hints from the server's `:validate`.
export function SizesEditor({
  value,
  onChange,
  requirements,
  sizeTables,
  diskTypes,
  issues,
  disabled,
  autoRoles,
}: {
  value: RoleSizes
  onChange: (v: RoleSizes) => void
  requirements: Requirements | undefined
  // role → available presets (from the catalog provider); falls back to `db`.
  sizeTables: Record<string, SizeSpec[]> | undefined
  diskTypes?: { id: string; title?: string; min_gb?: number }[]
  issues?: Issue[]
  disabled?: boolean
  /** Roles whose size was picked automatically (marked until changed). */
  autoRoles?: readonly string[]
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const roles = [
    ...new Set([...Object.keys(requirements ?? {}), ...Object.keys(value), 'db', 'runner']),
  ].sort((a, b) => roleRank(a) - roleRank(b) || a.localeCompare(b))
  const tableFor = (role: string): SizeSpec[] => sizeTables?.[role] ?? sizeTables?.db ?? []
  const setRole = (role: string, patch: Partial<RoleSizes[string]>) =>
    onChange({
      ...value,
      [role]: { ...value[role], size: patch.size ?? value[role]?.size ?? 'M', ...patch },
    })
  const issuesFor = (role: string) =>
    (issues ?? []).filter((i) => i.path.startsWith(`sizes.${role}.`))

  return (
    <div className={styles.grid}>
      <div className={styles.head}>{t('library.sizes.role')}</div>
      <div className={styles.head}>{t('library.sizes.size')}</div>
      <div className={styles.head}>{t('library.sizes.disk')}</div>
      {roles.map((role) => {
        const req = requirements?.[role]
        const table = tableFor(role)
        const chosen = value[role]
        const spec = table.find((s) => s.size === chosen?.size)
        const fits =
          !!spec && !!req && spec.cpu >= (req.cpu ?? 0) && spec.memory_gb >= (req.memory_gb ?? 0)
        const roleIssues = issuesFor(role)
        const showDisk = role === 'db' || (req?.disk_gb ?? 0) > 0 || chosen?.disk !== undefined
        return (
          <div key={role} style={{ display: 'contents' }}>
            <div className={styles.role} title={role}>
              {roleLabel(t, role)}
              {autoRoles?.includes(role) && (
                <Tooltip content={t('library.sizes.autoHint')}>
                  <span className={styles.auto}>{t('library.sizes.auto')}</span>
                </Tooltip>
              )}
              {req && (
                <div className={styles.req}>
                  ≥ {req.cpu ?? 0} vCPU · {req.memory_gb ?? 0} GB
                  {req.disk_gb ? ` · ${req.disk_gb} GB` : ''}
                </div>
              )}
            </div>
            <div className={styles.cell}>
              <Combobox<Size>
                id={`size-${role}`}
                disabled={disabled}
                placeholder={t('library.sizes.pick')}
                options={table.map((s) => ({
                  label: `${s.size} · ${s.cpu} CPU / ${s.memory_gb} GB`,
                  value: s.size,
                  description: s.instance_type,
                }))}
                value={chosen?.size ?? null}
                onChange={(o) => setRole(role, { size: o.value })}
              />
              {chosen && req && (
                <div className={`${styles.issue} ${fits ? styles.ok : styles.error}`}>
                  <Icon name={fits ? 'check' : 'exclamation-circle'} size="sm" />
                  {fits ? t('library.sizes.fits') : t('library.sizes.tooSmall')}
                </div>
              )}
              {roleIssues.map((i) => (
                <div
                  key={`${i.path}-${i.code}`}
                  className={`${styles.issue} ${i.severity === 'ERROR' ? styles.error : styles.warning}`}
                >
                  <Icon
                    name={i.severity === 'ERROR' ? 'exclamation-circle' : 'exclamation-triangle'}
                    size="sm"
                  />
                  <span>{i.message ?? i.code}</span>
                  {i.suggested !== undefined && i.suggested !== null && !disabled && (
                    <Button
                      size="sm"
                      variant="secondary"
                      fill="text"
                      onClick={() =>
                        i.path.endsWith('.disk.gb')
                          ? setRole(role, { disk: { ...chosen?.disk, gb: Number(i.suggested) } })
                          : setRole(role, {
                              size:
                                suggestedRoleSize(i.suggested)?.size ??
                                (String(i.suggested) as Size),
                            })
                      }
                    >
                      {t('library.sizes.use', { value: suggestedText(i.suggested) })}
                    </Button>
                  )}
                </div>
              ))}
            </div>
            <div>
              {showDisk ? (
                <Stack gap={0.5} alignItems="center">
                  {diskTypes && diskTypes.length > 0 && (
                    <Combobox
                      width={18}
                      disabled={disabled}
                      placeholder={t('library.sizes.diskType')}
                      options={diskTypes.map((d) => ({ label: d.title ?? d.id, value: d.id }))}
                      value={chosen?.disk?.type ?? diskTypes[0].id}
                      onChange={(o) => setRole(role, { disk: { ...chosen?.disk, type: o.value } })}
                    />
                  )}
                  <Input
                    type="number"
                    width={10}
                    min={10}
                    step={10}
                    disabled={disabled}
                    suffix="GB"
                    placeholder={String(spec?.default_disk_gb ?? req?.disk_gb ?? 100)}
                    value={chosen?.disk?.gb ?? ''}
                    onChange={(e) =>
                      setRole(role, {
                        disk: {
                          ...chosen?.disk,
                          gb:
                            e.currentTarget.value === ''
                              ? undefined
                              : Number(e.currentTarget.value),
                        },
                      })
                    }
                  />
                </Stack>
              ) : (
                <Tooltip content={t('library.sizes.noDiskHint')}>
                  <Text color="secondary">—</Text>
                </Tooltip>
              )}
            </div>
          </div>
        )
      })}
    </div>
  )
}
