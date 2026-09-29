import { css, cx } from '@emotion/css'
import type { GrafanaTheme2, IconName } from '@grafana/data'
import { Icon, IconButton, Tooltip, useStyles2 } from '@grafana/ui'
import { useTenant } from '@hooks/useTenant'
import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { AppLink } from './AppLink'

export const SIDEBAR_RAIL = 56
export const SIDEBAR_EXPANDED = 232
const KEY = 'stroppy.sidebar'
const TRANSITION_MS = 150

interface Item {
  id: string
  labelKey: string
  icon: IconName
  to: string
  exact?: boolean
  admin?: boolean
}
interface Section {
  id: string
  labelKey?: string
  items: Item[]
}

// Order follows how often things are touched: tests first, then execution, library, results.
const SECTIONS: Section[] = [
  {
    id: 'overview',
    items: [{ id: 'dashboard', labelKey: 'dashboard', icon: 'apps', to: '/t/$slug', exact: true }],
  },
  {
    id: 'scenarios',
    labelKey: 'sectionScenarios',
    items: [
      { id: 'tests', labelKey: 'tests', icon: 'vial', to: '/t/$slug/library/tests' },
      { id: 'suites', labelKey: 'suites', icon: 'layer-group', to: '/t/$slug/suites' },
    ],
  },
  {
    id: 'execution',
    labelKey: 'sectionExecution',
    items: [
      { id: 'runs', labelKey: 'runs', icon: 'play', to: '/t/$slug/runs' },
      { id: 'schedules', labelKey: 'schedules', icon: 'clock-nine', to: '/t/$slug/schedules' },
    ],
  },
  {
    id: 'library',
    labelKey: 'sectionLibrary',
    items: [
      {
        id: 'databases',
        labelKey: 'databases',
        icon: 'database',
        to: '/t/$slug/library/databases',
      },
      { id: 'workloads', labelKey: 'workloads', icon: 'bolt', to: '/t/$slug/library/workloads' },
    ],
  },
  {
    id: 'results',
    labelKey: 'sectionResults',
    items: [
      { id: 'compare', labelKey: 'compare', icon: 'columns', to: '/t/$slug/compare' },
      { id: 'rating', labelKey: 'rating', icon: 'star', to: '/t/$slug/rating' },
      { id: 'shares', labelKey: 'shares', icon: 'share-alt', to: '/t/$slug/shares' },
    ],
  },
]

const BOTTOM: Item[] = [
  { id: 'examples', labelKey: 'examples', icon: 'book-open', to: '/examples' },
  { id: 'settings', labelKey: 'settings', icon: 'cog', to: '/t/$slug/settings' },
  { id: 'admin', labelKey: 'admin', icon: 'shield', to: '/admin', admin: true },
]

const getStyles = (theme: GrafanaTheme2) => ({
  // Reserves the column in the flex layout: rail width, or the full width when pinned.
  slot: css({
    flexShrink: 0,
    position: 'relative',
    transition: `width ${TRANSITION_MS}ms ease`,
  }),
  aside: css({
    position: 'absolute',
    top: 0,
    bottom: 0,
    left: 0,
    display: 'flex',
    flexDirection: 'column',
    background: theme.colors.background.primary,
    borderRight: `1px solid ${theme.colors.border.weak}`,
    userSelect: 'none',
    overflow: 'hidden',
    zIndex: theme.zIndex.sidemenu,
    transition: `width ${TRANSITION_MS}ms ease, box-shadow ${TRANSITION_MS}ms ease`,
  }),
  overlay: css({ boxShadow: theme.shadows.z3 }),
  scroll: css({
    flex: 1,
    minHeight: 0,
    overflowY: 'auto',
    overflowX: 'hidden',
    padding: theme.spacing(1, 0),
  }),
  bottom: css({
    marginTop: 'auto',
    flexShrink: 0,
    borderTop: `1px solid ${theme.colors.border.weak}`,
    padding: theme.spacing(1, 0, 0.5),
  }),
  section: css({ padding: theme.spacing(0, 1) }),
  sectionDivider: css({
    height: 1,
    background: theme.colors.border.weak,
    margin: theme.spacing(1.5, 0.5, 0.5),
  }),
  // Section header: small uppercase muted caption; items are indented beneath it.
  sectionLabel: css({
    height: 24,
    display: 'flex',
    alignItems: 'center',
    padding: theme.spacing(0, 1),
    fontSize: 11,
    lineHeight: 1,
    textTransform: 'uppercase',
    letterSpacing: 0.6,
    color: theme.colors.text.disabled,
    fontWeight: theme.typography.fontWeightMedium,
    whiteSpace: 'nowrap',
    overflow: 'hidden',
  }),
  items: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(0.25) }),
  itemsIndented: css({ paddingLeft: theme.spacing(1) }),
  item: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    height: 32,
    padding: theme.spacing(0, 1),
    borderRadius: theme.shape.radius.default,
    color: theme.colors.text.secondary,
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    '&:hover': {
      color: theme.colors.text.primary,
      background: theme.colors.action.hover,
      textDecoration: 'none',
    },
    '&[data-status="active"]': {
      color: theme.colors.text.primary,
      background: theme.colors.action.selected,
      fontWeight: theme.typography.fontWeightMedium,
      boxShadow: `inset 3px 0 0 ${theme.colors.primary.main}`,
    },
  }),
  itemRail: css({ justifyContent: 'center', padding: 0, width: SIDEBAR_RAIL - 16 }),
  itemIcon: css({ flexShrink: 0 }),
  label: css({ overflow: 'hidden', textOverflow: 'ellipsis' }),
  pinRow: css({
    display: 'flex',
    justifyContent: 'flex-end',
    padding: theme.spacing(0.5, 1.5, 0.5),
  }),
  pinRowRail: css({ justifyContent: 'center', padding: theme.spacing(0.5, 0) }),
})

function readPinned(): boolean {
  try {
    const v = JSON.parse(localStorage.getItem(KEY) ?? '')
    return !!v?.pinned
  } catch {
    return false
  }
}

// Left navigation, Supabase-console style: a 56px icon rail that expands to 232px on hover or
// keyboard focus (overlaying the page), and a pin toggle that keeps it expanded and pushes the
// content instead. Only the pinned flag is persisted (localStorage `stroppy.sidebar`).
export function Sidebar() {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const { slug, me, tenant } = useTenant()
  const [pinned, setPinned] = useState(readPinned)
  const [hovered, setHovered] = useState(false)
  const [focused, setFocused] = useState(false)

  useEffect(() => {
    try {
      localStorage.setItem(KEY, JSON.stringify({ pinned }))
    } catch {
      // ignore
    }
  }, [pinned])

  const expanded = pinned || hovered || focused
  const overlay = expanded && !pinned

  const onBlur = useCallback((e: React.FocusEvent<HTMLElement>) => {
    if (!e.currentTarget.contains(e.relatedTarget as Node | null)) setFocused(false)
  }, [])

  const renderItem = (it: Item) => {
    if (it.admin && !me.is_platform_admin) return null
    if (!tenant && it.to.includes('$slug')) return null
    const label = t(`nav.${it.labelKey}`)
    const link = (
      <AppLink
        key={it.id}
        to={it.to as never}
        params={{ slug } as never}
        plain
        className={cx(styles.item, !expanded && styles.itemRail)}
        activeOptions={{ exact: it.exact, includeSearch: false }}
        aria-label={label}
      >
        <Icon name={it.icon} size="md" className={styles.itemIcon} />
        {expanded && <span className={styles.label}>{label}</span>}
      </AppLink>
    )
    return expanded ? (
      link
    ) : (
      <Tooltip key={it.id} content={label} placement="right">
        {link}
      </Tooltip>
    )
  }

  const pinLabel = pinned ? t('nav.unpin') : t('nav.pin')

  return (
    <div className={styles.slot} style={{ width: pinned ? SIDEBAR_EXPANDED : SIDEBAR_RAIL }}>
      <aside
        className={cx(styles.aside, overlay && styles.overlay)}
        style={{ width: expanded ? SIDEBAR_EXPANDED : SIDEBAR_RAIL }}
        aria-label="primary"
        data-expanded={expanded}
        onMouseEnter={() => setHovered(true)}
        onMouseLeave={() => setHovered(false)}
        onFocus={() => setFocused(true)}
        onBlur={onBlur}
      >
        <div className={styles.scroll}>
          {tenant &&
            SECTIONS.map((s, i) => (
              <div key={s.id} className={styles.section}>
                {i > 0 && <div className={styles.sectionDivider} />}
                {s.labelKey && expanded && (
                  <div className={styles.sectionLabel}>{t(`nav.${s.labelKey}`)}</div>
                )}
                <div className={cx(styles.items, expanded && s.labelKey && styles.itemsIndented)}>
                  {s.items.map(renderItem)}
                </div>
              </div>
            ))}
        </div>
        <div className={styles.bottom}>
          <div className={styles.section}>
            <div className={styles.items}>{BOTTOM.map(renderItem)}</div>
          </div>
          <div className={cx(styles.pinRow, !expanded && styles.pinRowRail)}>
            <IconButton
              name={pinned ? 'columns' : 'angle-double-right'}
              tooltip={pinLabel}
              tooltipPlacement="right"
              aria-pressed={pinned}
              onClick={() => setPinned((p) => !p)}
            />
          </div>
        </div>
      </aside>
    </div>
  )
}
