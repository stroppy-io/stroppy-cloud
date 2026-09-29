// GitHub-style keyboard navigation, registered once in AppShell.
export interface HotkeyDef {
  keys: string
  // i18n key under nav.*
  labelKey: string
  group: 'goto' | 'global'
}

export const HOTKEYS: HotkeyDef[] = [
  { keys: 'g d', labelKey: 'dashboard', group: 'goto' },
  { keys: 'g r', labelKey: 'runs', group: 'goto' },
  { keys: 'g t', labelKey: 'tests', group: 'goto' },
  { keys: 'g b', labelKey: 'databases', group: 'goto' },
  { keys: 'g w', labelKey: 'workloads', group: 'goto' },
  { keys: 'g u', labelKey: 'suites', group: 'goto' },
  { keys: 'g s', labelKey: 'settings', group: 'goto' },
  { keys: '/', labelKey: 'shortcutSearch', group: 'global' },
  { keys: 'n', labelKey: 'shortcutNewTest', group: 'global' },
  { keys: '?', labelKey: 'shortcutHelp', group: 'global' },
  { keys: 'Escape', labelKey: 'shortcutClose', group: 'global' },
]
