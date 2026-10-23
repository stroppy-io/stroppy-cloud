import { DeckCue, DeckNav } from '@components/landing/DeckNav'
import { HEADER_HEIGHT, PublicHeader } from '@components/landing/PublicHeader'
import { ARCH_STEPS, ArchitectureDiagram } from '@components/public/ArchitectureDiagram'
import { ConceptDiagram, PRODUCT_STEPS } from '@components/public/ConceptDiagram'
import { PageHero } from '@components/public/PageHero'
import { css, cx } from '@emotion/css'
import { type GrafanaTheme2, ThemeContext } from '@grafana/data'
import { useStyles2 } from '@grafana/ui'
import { useStepDeck } from '@hooks/useStepDeck'
import { AnimatePresence, motion, useReducedMotion } from 'motion/react'
import { type ReactNode, useLayoutEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { brand } from '@/styles/brand'
import { getAppTheme } from '@/styles/theme'
import { RunLifecycle } from './RunLifecycle'

// How Stroppy Cloud works, two screens in a deck like the landing: the
// product (what you work with) and the internals (how a run goes through the
// system, one path at a time via the chips).

const DARK = getAppTheme('dark')
const SCREENS = ['model', 'arch'] as const
const HASHES = ['', 'internals'] as const

const WIDE = '@media (min-width: 1200px)'

const getStyles = (theme: GrafanaTheme2) => ({
  root: css({
    height: '100vh',
    overflow: 'hidden',
    background: brand.ink,
    color: brand.text,
    fontFamily: theme.typography.fontFamily,
    overscrollBehavior: 'none',
  }),
  stage: css({
    position: 'relative',
    height: `calc(100vh - ${HEADER_HEIGHT}px)`,
    overflow: 'hidden',
  }),
  screen: css({ position: 'absolute', inset: 0 }),
  // The screen's content, fitted to the viewport: scaled down, never cut.
  slide: css({
    maxWidth: 1760,
    margin: '0 auto',
    padding: theme.spacing(12, 8, 10),
    transformOrigin: 'top center',
    [theme.breakpoints.down('md')]: { padding: theme.spacing(4, 2, 8) },
    // Wide screens are short for their width: the heading moves to a side
    // column and the diagram gets the whole height.
    [WIDE]: {
      display: 'grid',
      gridTemplateColumns: 'clamp(260px, 22vw, 380px) minmax(0, 1fr)',
      gap: theme.spacing(6),
      alignItems: 'start',
      padding: theme.spacing(12, 6, 8),
    },
  }),
  aside: css({
    [WIDE]: {
      '& h1': { fontSize: 'clamp(28px, 2.3vw, 44px)' },
      '& h1 + p': { fontSize: 'clamp(14px, 1vw, 18px)' },
      '& header': { marginBottom: theme.spacing(3) },
    },
  }),
  archHead: css({
    display: 'grid',
    gridTemplateColumns: 'minmax(0, 1fr) minmax(0, 1.4fr)',
    gap: theme.spacing(4),
    alignItems: 'start',
    marginBottom: theme.spacing(3),
    [theme.breakpoints.down('lg')]: { gridTemplateColumns: 'minmax(0, 1fr)' },
    [WIDE]: { gridTemplateColumns: 'minmax(0, 1fr)', gap: theme.spacing(2.5), marginBottom: 0 },
  }),
  tabs: css({
    display: 'flex',
    flexWrap: 'wrap',
    gap: theme.spacing(0.75),
    margin: 0,
    padding: 0,
    border: 'none',
  }),
  tab: css({
    display: 'inline-flex',
    alignItems: 'center',
    gap: theme.spacing(0.75),
    height: 30,
    padding: theme.spacing(0, 1.25),
    borderRadius: theme.shape.radius.pill,
    border: `1px solid ${brand.line}`,
    background: 'none',
    color: brand.muted,
    cursor: 'pointer',
    fontSize: theme.typography.bodySmall.fontSize,
    '&:hover': { color: brand.text, borderColor: brand.muted },
    '&:focus-visible': { outline: `2px solid ${brand.text}`, outlineOffset: 2 },
  }),
  tabActive: css({
    color: brand.text,
    background: brand.raised,
    borderColor: theme.colors.primary.border,
  }),
  tabNum: css({
    fontFamily: theme.typography.fontFamilyMonospace,
    color: theme.colors.primary.text,
  }),
  caption: css({ minHeight: 72 }),
  captionTitle: css({
    margin: 0,
    fontSize: theme.typography.h4.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
  }),
  captionBody: css({ margin: theme.spacing(0.75, 0, 0), color: brand.muted, lineHeight: 1.55 }),
})

// A screen's heading: its title, the paths through its diagram as chips,
// and the caption of the chosen one.
function StepsHead({
  prefix,
  steps,
  current,
  onPick,
}: {
  prefix: 'how.model' | 'how.arch'
  steps: readonly string[]
  current: number
  onPick: (i: number) => void
}) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const reduce = useReducedMotion() ?? false
  return (
    <div className={styles.archHead}>
      <div>
        <fieldset className={styles.tabs} aria-label={t(`${prefix}.title`)}>
          {steps.map((s, i) => (
            <button
              key={s}
              type="button"
              className={cx(styles.tab, i === current && styles.tabActive)}
              aria-pressed={i === current}
              onClick={() => onPick(i)}
            >
              {i > 0 && <span className={styles.tabNum}>{i}</span>}
              {t(`${prefix}.steps.${s}.title`)}
            </button>
          ))}
        </fieldset>
      </div>
      <div className={styles.caption} aria-live="polite">
        <AnimatePresence mode="wait" initial={false}>
          <motion.div
            key={current}
            initial={{ opacity: 0, y: 6 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -6 }}
            transition={reduce ? { duration: 0 } : { duration: 0.2 }}
          >
            <h3 className={styles.captionTitle}>{t(`${prefix}.steps.${steps[current]}.title`)}</h3>
            <p className={styles.captionBody}>{t(`${prefix}.steps.${steps[current]}.body`)}</p>
          </motion.div>
        </AnimatePresence>
      </div>
    </div>
  )
}

// Scales a screen's content down to the stage when it is taller (a laptop):
// the deck has no scroll, so nothing may fall below the fold.
function Fit({ className, children }: { className: string; children: ReactNode }) {
  const ref = useRef<HTMLDivElement>(null)
  const [k, setK] = useState(1)
  useLayoutEffect(() => {
    const el = ref.current
    const stage = el?.parentElement
    if (!el || !stage) return
    const measure = () => setK(Math.min(1, stage.clientHeight / el.offsetHeight))
    measure()
    const ro = new ResizeObserver(measure)
    ro.observe(el)
    ro.observe(stage)
    return () => ro.disconnect()
  }, [])
  return (
    <div ref={ref} className={className} style={k < 1 ? { transform: `scale(${k})` } : undefined}>
      {children}
    </div>
  )
}

function Deck() {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const reduce = useReducedMotion() ?? false
  const root = useRef<HTMLDivElement>(null)
  const { step, dir, go } = useStepDeck(root, SCREENS.length, { hashes: HASHES })
  const screen = SCREENS[step] ?? 'model'
  const [model, setModel] = useState(0)
  const [arch, setArch] = useState(0)
  const tr = reduce ? { duration: 0 } : { duration: 0.45, ease: [0.22, 1, 0.36, 1] as const }

  const labels = [t('how.deck.model'), t('how.deck.arch')]

  return (
    <div ref={root} className={styles.root}>
      <PublicHeader />
      <div className={styles.stage}>
        <AnimatePresence initial={false} custom={dir}>
          <motion.div
            key={screen}
            className={styles.screen}
            custom={dir}
            variants={{
              enter: (d: number) => ({ y: d > 0 ? 80 : -80, opacity: 0 }),
              center: { y: 0, opacity: 1 },
              exit: (d: number) => ({ y: d > 0 ? -80 : 80, opacity: 0 }),
            }}
            initial="enter"
            animate="center"
            exit="exit"
            transition={tr}
          >
            {screen === 'model' && (
              <Fit className={styles.slide}>
                <div className={styles.aside}>
                  <PageHero title={t('how.model.title')} lead={t('how.model.lead')} />
                  <StepsHead
                    prefix="how.model"
                    steps={PRODUCT_STEPS}
                    current={model}
                    onPick={setModel}
                  />
                </div>
                <div>
                  <ConceptDiagram step={model} hint={false} />
                  <RunLifecycle />
                </div>
              </Fit>
            )}
            {screen === 'arch' && (
              <Fit className={styles.slide}>
                <div className={styles.aside}>
                  <PageHero title={t('how.arch.title')} lead={t('how.arch.lead')} />
                  <StepsHead prefix="how.arch" steps={ARCH_STEPS} current={arch} onPick={setArch} />
                </div>
                <div>
                  <ArchitectureDiagram step={arch} />
                </div>
              </Fit>
            )}
          </motion.div>
        </AnimatePresence>
        <DeckNav labels={labels} current={step} onGo={go} />
        {step === 0 ? (
          <DeckCue key="down" direction="down" label={labels[1]} onGo={() => go(1)} />
        ) : (
          <DeckCue key="up" direction="up" label={labels[0]} onGo={() => go(0)} />
        )}
      </div>
    </div>
  )
}

export function HowPage() {
  return (
    <ThemeContext.Provider value={DARK}>
      <Deck />
    </ThemeContext.Provider>
  )
}
