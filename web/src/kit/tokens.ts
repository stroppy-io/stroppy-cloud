import raw from './tokens.json'

// The Stroppy look as values: one source for every place that has to look
// like Stroppy (the app's Grafana theme, generated CSS variables, a report
// page). tokens.json is the source so that anything can read it, not only
// TypeScript.

export type Mode = 'dark' | 'light'

export interface Palette {
  /** The page behind everything. */
  canvas: string
  /** Panels, cards, inputs. */
  panel: string
  /** Something lifted from a panel: hover, a header row, a nested box. */
  raised: string
  /** Dividers and borders. */
  line: string
  /** Borders that must be seen: focused or hovered controls. */
  lineStrong: string
  text: string
  /** Secondary text: hints, captions, units. */
  muted: string
  disabled: string
  /** Actions and progress. */
  primary: string
  /** Primary as text and links: readable on canvas and panel. */
  primaryText: string
  success: string
  warning: string
  error: string
  info: string
}

export interface Tokens {
  font: {
    sans: string
    mono: string
    size: { sm: string; base: string; lg: string }
    weight: { regular: number; medium: number; bold: number }
    lineHeight: number
  }
  /** The spacing unit; everything spaces in multiples of it. */
  space: string
  radius: { sm: string; md: string; lg: string; pill: string }
  /** The orange of the logo mark: rays and band. Same in both modes. */
  mark: string
  color: Record<Mode, Palette>
  /** Categorical colours for chart series and legends, in order of use. */
  series: Record<Mode, string[]>
}

export const tokens: Tokens = raw
