import { createTheme, type GrafanaTheme2 } from '@grafana/data'
import { type Mode, tokens } from '@/kit/tokens'

export type ThemeMode = Mode

const cache = new Map<ThemeMode, GrafanaTheme2>()

export function getAppTheme(mode: ThemeMode): GrafanaTheme2 {
  let theme = cache.get(mode)
  if (!theme) {
    theme = createTheme({ colors: { mode } })
    cache.set(mode, theme)
  }
  return theme
}

const px = (v: string) => Number.parseInt(v, 10)

/**
 * Grafana's theme in the Stroppy look: palette, fonts and radii from the kit
 * tokens. Grafana derives the rest (hover shades, contrast text, shadows)
 * from these.
 */
export function createStroppyTheme(mode: ThemeMode): GrafanaTheme2 {
  const c = tokens.color[mode]
  const f = tokens.font
  return createTheme({
    colors: {
      mode,
      primary: { main: c.primary, text: c.primaryText },
      info: { main: c.info },
      success: { main: c.success },
      warning: { main: c.warning },
      error: { main: c.error },
      text: {
        primary: c.text,
        secondary: c.muted,
        disabled: c.disabled,
        link: c.primaryText,
        maxContrast: mode === 'dark' ? '#ffffff' : '#000000',
      },
      background: { canvas: c.canvas, primary: c.panel, secondary: c.raised, elevated: c.raised },
      border: { weak: c.line, medium: c.lineStrong, strong: c.muted },
    },
    typography: {
      fontFamily: f.sans,
      fontFamilyMonospace: f.mono,
      fontSize: px(f.size.base),
      fontWeightRegular: f.weight.regular,
      fontWeightMedium: f.weight.medium,
      fontWeightBold: f.weight.bold,
    },
    shape: {
      borderRadiusSm: px(tokens.radius.sm),
      borderRadius: px(tokens.radius.md),
      borderRadiusLg: px(tokens.radius.lg),
    },
  })
}
