import { createContext, useContext } from 'react'
import type { ThemeMode } from '@/styles/theme'

export const ThemeModeContext = createContext<{ mode: ThemeMode; setMode: (m: ThemeMode) => void }>(
  { mode: 'dark', setMode: () => undefined }
)

export function useThemeMode() {
  return useContext(ThemeModeContext)
}
