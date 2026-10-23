import type { GrafanaTheme2 } from '@grafana/data'
import { tokens } from '@/kit/tokens'

// The public face of Stroppy Cloud (sign-in, onboarding, the public rating)
// carries the stroppy.io identity rather than the Grafana app theme: the kit
// palette of the current mode; actions and progress use the app's primary
// colour. The app itself stays on GrafanaTheme2; only public pages read these.
export interface Brand {
  ink: string
  panel: string
  raised: string
  line: string
  text: string
  muted: string
  signal: string
  good: string
}

const of = (mode: 'dark' | 'light'): Brand => {
  const c = tokens.color[mode]
  return {
    ink: c.canvas,
    panel: c.panel,
    raised: c.raised,
    line: c.line,
    text: c.text,
    muted: c.muted,
    signal: c.primary,
    good: c.success,
  }
}

const BRANDS = { dark: of('dark'), light: of('light') }

/** The brand palette of the theme's mode. */
export function brandOf(theme: GrafanaTheme2): Brand {
  return theme.isDark ? BRANDS.dark : BRANDS.light
}
