import 'driver.js/dist/driver.css'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { useStyles2, useTheme2 } from '@grafana/ui'
import { useOnboarding } from '@hooks/useOnboarding'
import { type DriveStep, driver } from 'driver.js'
import { useCallback, useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'

// The interface tour: driver.js does the highlighting and positioning, the
// popover is restyled here with the Grafana theme. Steps point at
// `data-tour` anchors of the shell (sidebar items, search, tenant switcher,
// user menu); a missing anchor (role, no tenant) drops its step.

// In the order of the sidebar, then the header left to right.
const STEPS: Array<{ id: string; anchor: string; side: 'right' | 'bottom' }> = [
  { id: 'tests', anchor: 'nav-tests', side: 'right' },
  { id: 'suites', anchor: 'nav-suites', side: 'right' },
  { id: 'runs', anchor: 'nav-runs', side: 'right' },
  { id: 'databases', anchor: 'nav-databases', side: 'right' },
  { id: 'workloads', anchor: 'nav-workloads', side: 'right' },
  { id: 'examples', anchor: 'nav-examples', side: 'right' },
  { id: 'settings', anchor: 'nav-settings', side: 'right' },
  { id: 'search', anchor: 'search', side: 'bottom' },
  { id: 'tenant', anchor: 'tenant', side: 'bottom' },
  { id: 'userMenu', anchor: 'user-menu', side: 'bottom' },
]

const getStyles = (theme: GrafanaTheme2) => ({
  popover: css({
    '&.driver-popover': {
      background: theme.colors.background.primary,
      color: theme.colors.text.primary,
      border: `1px solid ${theme.colors.border.medium}`,
      borderRadius: theme.shape.radius.default,
      boxShadow: theme.shadows.z3,
      padding: theme.spacing(2),
      maxWidth: 360,
      fontFamily: theme.typography.fontFamily,
    },
    '.driver-popover-title': {
      fontSize: theme.typography.h5.fontSize,
      fontWeight: theme.typography.fontWeightMedium,
      fontFamily: theme.typography.fontFamily,
      color: theme.colors.text.primary,
    },
    '.driver-popover-description': {
      fontSize: theme.typography.body.fontSize,
      lineHeight: theme.typography.body.lineHeight,
      color: theme.colors.text.secondary,
      fontFamily: theme.typography.fontFamily,
    },
    '.driver-popover-progress-text': {
      color: theme.colors.text.secondary,
      fontSize: theme.typography.bodySmall.fontSize,
    },
    '.driver-popover-close-btn': { color: theme.colors.text.secondary },
    '.driver-popover-close-btn:focus': { outline: 'none' },
    '.driver-popover-close-btn:focus-visible': {
      outline: `2px solid ${theme.colors.primary.border}`,
      outlineOffset: 1,
    },
    // "Back" on the first step has nowhere to go.
    '.driver-popover-navigation-btns .driver-popover-btn-disabled': { display: 'none' },
    '.driver-popover-close-btn:hover': { color: theme.colors.text.primary },
    '.driver-popover-navigation-btns button': {
      textShadow: 'none',
      fontFamily: theme.typography.fontFamily,
      fontSize: theme.typography.bodySmall.fontSize,
      fontWeight: theme.typography.fontWeightMedium,
      padding: theme.spacing(0.5, 1.5),
      borderRadius: theme.shape.radius.default,
      border: `1px solid ${theme.colors.border.medium}`,
      background: theme.colors.secondary.main,
      color: theme.colors.secondary.text,
    },
    '.driver-popover-navigation-btns button:hover': { background: theme.colors.secondary.shade },
    '.driver-popover-navigation-btns .driver-popover-next-btn': {
      background: theme.colors.primary.main,
      borderColor: theme.colors.primary.main,
      color: theme.colors.primary.contrastText,
    },
    '.driver-popover-navigation-btns .driver-popover-next-btn:hover': {
      background: theme.colors.primary.shade,
    },
    // The arrow is drawn with borders in the popover colour.
    '.driver-popover-arrow-side-left.driver-popover-arrow': {
      borderLeftColor: theme.colors.background.primary,
    },
    '.driver-popover-arrow-side-right.driver-popover-arrow': {
      borderRightColor: theme.colors.background.primary,
    },
    '.driver-popover-arrow-side-top.driver-popover-arrow': {
      borderTopColor: theme.colors.background.primary,
    },
    '.driver-popover-arrow-side-bottom.driver-popover-arrow': {
      borderBottomColor: theme.colors.background.primary,
    },
  }),
})

/** Starts the interface tour; finishing or closing it marks it done for the account. */
export function useTour() {
  const { t } = useTranslation()
  const theme = useTheme2()
  const styles = useStyles2(getStyles)
  const onboarding = useOnboarding()
  const setRef = useRef(onboarding.set)
  setRef.current = onboarding.set

  return useCallback(() => {
    const steps: DriveStep[] = STEPS.filter((s) =>
      document.querySelector(`[data-tour="${s.anchor}"]`)
    ).map((s) => ({
      element: `[data-tour="${s.anchor}"]`,
      popover: {
        title: t(`onboarding.tour.${s.id}.title`),
        description: t(`onboarding.tour.${s.id}.body`),
        side: s.side,
        align: 'start',
      },
    }))
    if (!steps.length) return
    const tour = driver({
      steps,
      showProgress: true,
      // driver.js fills {{current}}/{{total}} itself: pass them through i18next as is.
      progressText: t('onboarding.tour.progress', {
        current: '{{current}}',
        total: '{{total}}',
        interpolation: { escapeValue: false },
      }),
      nextBtnText: t('onboarding.tour.next'),
      prevBtnText: t('onboarding.tour.prev'),
      doneBtnText: t('onboarding.tour.done'),
      popoverClass: styles.popover,
      // A dimmed page in either theme: the dark canvas, or dark text in the light one.
      overlayColor: theme.isDark ? theme.colors.background.canvas : theme.colors.text.primary,
      overlayOpacity: 0.6,
      stagePadding: 4,
      stageRadius: 4,
      onDestroyed: () => void setRef.current({ tour_done: true }),
    })
    tour.drive()
  }, [t, theme, styles])
}

/** Runs the tour once, on the first visit of a tenant after the welcome screen. */
export function TourAutoStart() {
  const onboarding = useOnboarding()
  const start = useTour()
  const started = useRef(false)
  useEffect(() => {
    if (started.current || !onboarding.welcomeDone || onboarding.tourDone) return
    started.current = true
    // Let the shell lay out its anchors first.
    const id = window.setTimeout(start, 600)
    return () => window.clearTimeout(id)
  }, [onboarding.welcomeDone, onboarding.tourDone, start])
  return null
}
