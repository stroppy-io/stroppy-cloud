import { DeckCue, DeckNav } from '@components/landing/DeckNav'
import { HEADER_HEIGHT, PublicHeader } from '@components/landing/PublicHeader'
import { ARCH_STEPS, ArchitectureDiagram } from '@components/public/ArchitectureDiagram'
import { ConceptDiagram } from '@components/public/ConceptDiagram'
import { css, cx } from '@emotion/css'
import { type GrafanaTheme2, ThemeContext } from '@grafana/data'
import { useStyles2 } from '@grafana/ui'
import { useStepDeck } from '@hooks/useStepDeck'
import { AnimatePresence, motion, useReducedMotion } from 'motion/react'
import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { brand } from '@/styles/brand'
import { getAppTheme } from '@/styles/theme'

// How Stroppy Cloud works, two screens in a deck like the landing: the
// product (what you work with) and the internals (how a run goes through the
// system, one path at a time via the chips).

const DARK = getAppTheme('dark')
const SCREENS = ['model', 'arch'] as const
const HASHES = ['', 'internals'] as const

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
  slide: css({
    height: '100%',
    maxWidth: 1280,
    margin: '0 auto',
    padding: theme.spacing(8, 8, 10),
    display: 'flex',
    flexDirection: 'column',
    justifyContent: 'center',
    [theme.breakpoints.down('md')]: { padding: theme.spacing(4, 2, 8) },
  }),
  h2: css({
    margin: 0,
    fontSize: 'clamp(26px, 2.4vw, 34px)',
    lineHeight: 1.15,
    fontWeight: 700,
    letterSpacing: '-0.02em',
  }),
  sub: css({ margin: theme.spacing(1, 0, 3), color: brand.muted, maxWidth: 760, lineHeight: 1.5 }),
  archHead: css({
    display: 'grid',
    gridTemplateColumns: 'minmax(0, 1fr) minmax(0, 1.4fr)',
    gap: theme.spacing(4),
    alignItems: 'end',
    marginBottom: theme.spacing(3),
    [theme.breakpoints.down('lg')]: { gridTemplateColumns: 'minmax(0, 1fr)' },
  }),
  tabs: css({
    display: 'flex',
    flexWrap: 'wrap',
    gap: theme.spacing(0.75),
    marginTop: theme.spacing(2),
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

function Deck() {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const reduce = useReducedMotion() ?? false
  const root = useRef<HTMLDivElement>(null)
  const { step, dir, go } = useStepDeck(root, SCREENS.length, { hashes: HASHES })
  const screen = SCREENS[step] ?? 'model'
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
              <div className={styles.slide}>
                <h2 className={styles.h2}>{t('how.model.title')}</h2>
                <p className={styles.sub}>{t('how.model.sub')}</p>
                <ConceptDiagram />
              </div>
            )}
            {screen === 'arch' && (
              <div className={styles.slide}>
                <div className={styles.archHead}>
                  <div>
                    <h2 className={styles.h2}>{t('how.arch.title')}</h2>
                    <fieldset className={styles.tabs} aria-label={t('how.arch.title')}>
                      {ARCH_STEPS.map((s, i) => (
                        <button
                          key={s}
                          type="button"
                          className={cx(styles.tab, i === arch && styles.tabActive)}
                          aria-pressed={i === arch}
                          onClick={() => setArch(i)}
                        >
                          {i > 0 && <span className={styles.tabNum}>{i}</span>}
                          {t(`how.arch.steps.${s}.title`)}
                        </button>
                      ))}
                    </fieldset>
                  </div>
                  <div className={styles.caption} aria-live="polite">
                    <AnimatePresence mode="wait" initial={false}>
                      <motion.div
                        key={arch}
                        initial={{ opacity: 0, y: 6 }}
                        animate={{ opacity: 1, y: 0 }}
                        exit={{ opacity: 0, y: -6 }}
                        transition={reduce ? { duration: 0 } : { duration: 0.2 }}
                      >
                        <h3 className={styles.captionTitle}>
                          {t(`how.arch.steps.${ARCH_STEPS[arch]}.title`)}
                        </h3>
                        <p className={styles.captionBody}>
                          {t(`how.arch.steps.${ARCH_STEPS[arch]}.body`)}
                        </p>
                      </motion.div>
                    </AnimatePresence>
                  </div>
                </div>
                <ArchitectureDiagram step={arch} />
              </div>
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
