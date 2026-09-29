import type { Schemas } from '@api/types'

import { AppLink } from '@app/AppLink'
import { css } from '@emotion/css'
import type { GrafanaTheme2, IconName } from '@grafana/data'
import { Icon, Text, useStyles2 } from '@grafana/ui'
import { useTenant } from '@hooks/useTenant'
import type { LinkProps } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

type Usage = Schemas['Usage']

const ICON: Record<Usage['kind'], IconName> = {
  database: 'database',
  workload: 'bolt',
  test: 'vial',
  suite: 'layer-group',
  schedule: 'clock-nine',
  run: 'play',
}

export function usageLink(kind: Usage['kind']): LinkProps['to'] {
  switch (kind) {
    case 'database':
      return '/t/$slug/library/databases/$id'
    case 'workload':
      return '/t/$slug/library/workloads/$id'
    case 'test':
      return '/t/$slug/library/tests/$id'
    case 'suite':
      return '/t/$slug/suites/$id'
    case 'schedule':
      return '/t/$slug/schedules/$id'
    case 'run':
      return '/t/$slug/runs/$id'
  }
}

const getStyles = (theme: GrafanaTheme2) => ({
  list: css({ listStyle: 'none', margin: 0, padding: 0, display: 'grid', gap: theme.spacing(0.5) }),
  item: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    padding: theme.spacing(1, 1.5),
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.primary,
  }),
  kind: css({
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    minWidth: 72,
  }),
  group: css({
    fontSize: theme.typography.bodySmall.fontSize,
    color: theme.colors.text.secondary,
    textTransform: 'uppercase',
    letterSpacing: 0.4,
    margin: theme.spacing(1.5, 0, 0.5),
  }),
  empty: css({ color: theme.colors.text.secondary, padding: theme.spacing(2, 0) }),
})

// "Where is this used" — grouped by dependent kind, each row links to the record.
export function UsagesList({
  usages,
  emptyHint,
}: {
  usages: Usage[] | undefined
  emptyHint?: string
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const { slug } = useTenant()
  if (!usages?.length)
    return (
      <div className={styles.empty}>
        <Text color="secondary">{emptyHint ?? t('library.usages.empty')}</Text>
      </div>
    )
  const kinds = [...new Set(usages.map((u) => u.kind))]
  return (
    <div>
      {kinds.map((k) => (
        <div key={k}>
          <div className={styles.group}>
            {t(`library.usages.kind.${k}`)} · {usages.filter((u) => u.kind === k).length}
          </div>
          <ul className={styles.list}>
            {usages
              .filter((u) => u.kind === k)
              .map((u) => (
                <li key={`${u.kind}-${u.id}`} className={styles.item}>
                  <Icon name={ICON[u.kind]} size="sm" />
                  <AppLink to={usageLink(u.kind)} params={{ slug, id: u.id }}>
                    {u.name}
                  </AppLink>
                </li>
              ))}
          </ul>
        </div>
      ))}
    </div>
  )
}
