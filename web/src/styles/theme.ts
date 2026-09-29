import { createTheme, type GrafanaTheme2 } from '@grafana/data'

export type ThemeMode = 'dark' | 'light'

const cache = new Map<ThemeMode, GrafanaTheme2>()

export function getAppTheme(mode: ThemeMode): GrafanaTheme2 {
  let theme = cache.get(mode)
  if (!theme) {
    theme = createTheme({ colors: { mode } })
    cache.set(mode, theme)
  }
  return theme
}
