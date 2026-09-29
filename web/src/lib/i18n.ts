import i18next from 'i18next'
import { initReactI18next } from 'react-i18next'

// Feature namespaces are merged under one resource tree per language:
// `t('runs.list.title')`, `t('common.actions.save')`. Every feature adds its own
// `locales/{ru,en}/<feature>.json` (one file per worker → no merge conflicts).
const ru = import.meta.glob('../locales/ru/*.json', { eager: true, import: 'default' }) as Record<
  string,
  object
>
const en = import.meta.glob('../locales/en/*.json', { eager: true, import: 'default' }) as Record<
  string,
  object
>

function merge(files: Record<string, object>): Record<string, object> {
  const out: Record<string, object> = {}
  for (const [path, mod] of Object.entries(files)) {
    const name = path
      .split('/')
      .pop()
      ?.replace(/\.json$/, '')
    if (name) out[name] = mod
  }
  return out
}

export const SUPPORTED_LANGS = ['en', 'ru'] as const
export type Lang = (typeof SUPPORTED_LANGS)[number]

const STORAGE_KEY = 'stroppy.lang'

export function getStoredLang(): Lang {
  try {
    const v = localStorage.getItem(STORAGE_KEY)
    if (v === 'ru' || v === 'en') return v
  } catch {
    // ignore
  }
  return navigator.language.startsWith('ru') ? 'ru' : 'en'
}

export function setLang(lang: Lang): void {
  try {
    localStorage.setItem(STORAGE_KEY, lang)
  } catch {
    // ignore
  }
  void i18next.changeLanguage(lang)
}

// The default i18next singleton is shared with @grafana/i18n (it passes defaultValue for
// its own strings), so one init serves both.
export const i18n = i18next
if (!i18next.isInitialized) {
  void i18next.use(initReactI18next).init({
    resources: { ru: { translation: merge(ru) }, en: { translation: merge(en) } },
    lng: getStoredLang(),
    fallbackLng: 'en',
    keySeparator: '.',
    nsSeparator: false,
    returnEmptyString: false,
    interpolation: { escapeValue: false },
  })
}
