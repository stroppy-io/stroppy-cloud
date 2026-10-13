import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { useStyles2 } from '@grafana/ui'
import { useTranslation } from 'react-i18next'
import { brand } from '@/styles/brand'

// The deck's own map: one mark per step, on the right edge, centred. Muted
// until hovered; the current step is the long bright one. A label shows on
// hover or focus, and a click goes straight there.

const getStyles = (theme: GrafanaTheme2) => ({
  nav: css({
    position: 'absolute',
    right: theme.spacing(2.5),
    top: 0,
    bottom: 0,
    margin: 'auto 0',
    height: 'fit-content',
    zIndex: 5,
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(0.5),
    opacity: 0.55,
    transition: 'opacity 200ms',
    '&:hover, &:focus-within': { opacity: 1 },
    [theme.breakpoints.down('md')]: { right: theme.spacing(1) },
  }),
  item: css({
    position: 'relative',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'flex-end',
    width: 24,
    height: 20,
    padding: 0,
    border: 'none',
    background: 'none',
    cursor: 'pointer',
    '&:focus-visible': { outline: `2px solid ${brand.text}`, outlineOffset: 2, borderRadius: 4 },
    '&:hover > span, &:focus-visible > span': { opacity: 1, transform: 'translateX(0)' },
  }),
  mark: css({
    width: 6,
    height: 6,
    borderRadius: 3,
    background: brand.muted,
    transition: 'height 250ms, background 250ms',
  }),
  markActive: css({ height: 18, background: brand.text }),
  label: css({
    position: 'absolute',
    right: 20,
    whiteSpace: 'nowrap',
    padding: theme.spacing(0.25, 1),
    borderRadius: theme.shape.radius.default,
    background: brand.raised,
    border: `1px solid ${brand.line}`,
    color: brand.text,
    fontSize: theme.typography.bodySmall.fontSize,
    pointerEvents: 'none',
    opacity: 0,
    transform: 'translateX(4px)',
    transition: 'opacity 150ms, transform 150ms',
  }),
})

export function DeckNav({
  keys,
  current,
  onGo,
}: {
  /** One i18n key per step, under `landing.nav.steps`. */
  keys: readonly string[]
  current: number
  onGo: (step: number) => void
}) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  return (
    <nav className={styles.nav} aria-label={t('landing.nav.steps.label')}>
      {keys.map((k, i) => (
        <button
          key={k}
          type="button"
          className={styles.item}
          aria-current={i === current ? 'step' : undefined}
          aria-label={t(`landing.nav.steps.${k}`)}
          onClick={() => onGo(i)}
        >
          <span className={styles.label}>{t(`landing.nav.steps.${k}`)}</span>
          <i className={cx(styles.mark, i === current && styles.markActive)} />
        </button>
      ))}
    </nav>
  )
}
