import { apiMode } from '@api/mode'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Badge, Tooltip, useStyles2 } from '@grafana/ui'
import { useTenant } from '@hooks/useTenant'
import { useHotkey, useHotkeySequence } from '@tanstack/react-hotkeys'
import { Outlet, useNavigate } from '@tanstack/react-router'
import { type ReactNode, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { AppLink } from './AppLink'
import { Breadcrumbs } from './Breadcrumbs'
import { useBreadcrumbs, useIsBare } from './crumbs'
import { GlobalSearch, type GlobalSearchHandle } from './GlobalSearch'
import { ShortcutsModal } from './ShortcutsModal'
import { Sidebar } from './Sidebar'
import { TenantSwitcher } from './TenantSwitcher'
import { UserMenu } from './UserMenu'

export const HEADER_HEIGHT = 48
export const CRUMB_BAR_HEIGHT = 36

const getStyles = (theme: GrafanaTheme2) => ({
  // The shell owns the viewport: header is fixed height, the body fills the rest and only the
  // content column scrolls. No page-level scrollbars in either direction.
  root: css({
    height: '100vh',
    display: 'flex',
    flexDirection: 'column',
    // `clip`, not `hidden`: a hidden box is still scrolled by focus/scrollIntoView (a tall menu
    // opening near the bottom would shift the whole shell up).
    overflow: 'clip',
    background: theme.colors.background.canvas,
  }),
  top: css({
    flexShrink: 0,
    zIndex: theme.zIndex.navbarFixed,
    background: theme.colors.background.primary,
    borderBottom: `1px solid ${theme.colors.border.weak}`,
  }),
  topInner: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(2),
    height: HEADER_HEIGHT,
    padding: theme.spacing(0, 2),
  }),
  brand: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    fontWeight: theme.typography.fontWeightMedium,
    color: theme.colors.text.primary,
    whiteSpace: 'nowrap',
    '&:hover': { textDecoration: 'none', color: theme.colors.text.primary },
  }),
  logo: css({
    width: 26,
    height: 26,
    borderRadius: theme.shape.radius.default,
    display: 'block',
    objectFit: 'cover',
  }),
  center: css({
    flex: 1,
    display: 'flex',
    justifyContent: 'center',
    minWidth: 0,
    [theme.breakpoints.down('md')]: { display: 'none' },
  }),
  right: css({ display: 'flex', alignItems: 'center', gap: theme.spacing(1), flexShrink: 0 }),
  body: css({
    display: 'flex',
    flex: 1,
    minHeight: 0,
    alignItems: 'stretch',
    position: 'relative',
  }),
  main: css({
    flex: 1,
    minWidth: 0,
    minHeight: 0,
    display: 'flex',
    flexDirection: 'column',
  }),
  crumbBar: css({
    flexShrink: 0,
    display: 'flex',
    alignItems: 'center',
    height: CRUMB_BAR_HEIGHT,
    padding: theme.spacing(0, 3),
    borderBottom: `1px solid ${theme.colors.border.weak}`,
    background: theme.colors.background.primary,
  }),
  content: css({
    flex: 1,
    minHeight: 0,
    overflow: 'auto',
    overscrollBehavior: 'contain',
  }),
})

// Shell: top bar (brand · global search · tenant switcher · user), left rail sidebar,
// router-driven breadcrumbs, independently scrolling content area.
export function AppShell({ children }: { children?: ReactNode }) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const { slug, tenant } = useTenant()
  const navigate = useNavigate()
  const searchRef = useRef<GlobalSearchHandle>(null)
  const [helpOpen, setHelpOpen] = useState(false)
  const bare = useIsBare()
  const crumbCount = useBreadcrumbs().length

  // GitHub-style navigation shortcuts. Sequences are ignored while typing in inputs by the library.
  const go = (to: string) => () => void navigate({ to: to as never, params: { slug } as never })
  useHotkeySequence(['G', 'D'], go('/t/$slug'))
  useHotkeySequence(['G', 'R'], go('/t/$slug/runs'))
  useHotkeySequence(['G', 'T'], go('/t/$slug/library/tests'))
  useHotkeySequence(['G', 'B'], go('/t/$slug/library/databases'))
  useHotkeySequence(['G', 'W'], go('/t/$slug/library/workloads'))
  useHotkeySequence(['G', 'U'], go('/t/$slug/suites'))
  useHotkeySequence(['G', 'S'], go('/t/$slug/settings'))
  useHotkey('N', go('/t/$slug/library/tests/new'))
  useHotkey('/', (e) => {
    e.preventDefault()
    searchRef.current?.focus()
  })
  useHotkey('?', () => setHelpOpen(true))

  return (
    <div className={styles.root}>
      <header className={styles.top}>
        <div className={styles.topInner}>
          <AppLink to="/" className={styles.brand}>
            <img src="/org-logo.png" alt="" className={styles.logo} width={26} height={26} />
            {t('common.appName')}
            {apiMode === 'mock' && (
              <Tooltip content={t('nav.mockModeHint')}>
                <span>
                  <Badge text={t('nav.mockMode')} color="orange" icon="bug" />
                </span>
              </Tooltip>
            )}
          </AppLink>
          <div className={styles.center}>{tenant && <GlobalSearch ref={searchRef} />}</div>
          <div className={styles.right}>
            {!bare && <TenantSwitcher />}
            <UserMenu onShortcuts={() => setHelpOpen(true)} />
          </div>
        </div>
      </header>
      <div className={styles.body}>
        {!bare && <Sidebar />}
        <main className={styles.main}>
          {/* A single crumb only repeats the page title: no empty bar for top-level pages. */}
          {!bare && crumbCount > 1 && (
            <div className={styles.crumbBar}>
              <Breadcrumbs />
            </div>
          )}
          <div className={styles.content} data-scroll-restoration-id="app-content">
            {children ?? <Outlet />}
          </div>
        </main>
      </div>
      <ShortcutsModal isOpen={helpOpen} onClose={() => setHelpOpen(false)} />
    </div>
  )
}
