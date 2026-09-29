import { initPluginTranslations } from '@grafana/i18n'
import i18next from 'i18next'
import { initReactI18next, setI18n } from 'react-i18next'
// Russian for @grafana/ui / @grafana/data built-in strings (TimeRangePicker, RefreshPicker, Select,
// Combobox, Modal, Pagination, …). Generated from Grafana's official ru-RU `grafana.json` for the
// exact keys the installed dist calls + `ru.overrides.json`; regenerate with
// `node scripts/gen-grafana-ru.mjs` after bumping @grafana/ui.
// English needs no bundle: Grafana passes the English default with every `t()`.
import grafanaRu from '../locales/grafana/ru.json'

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

// ONE i18next instance serves the app and Grafana: `i18next`/`react-i18next` are kept on the same
// major versions as `@grafana/i18n` so the package manager hoists a single copy (a nested copy would
// be a separate singleton that stays on en-US forever). Grafana keys are full paths in the default
// namespace (`grafana-ui.select.placeholder`, `time-picker.range-content.from-input`), so its bundle
// sits at the top of the `translation` tree next to our feature namespaces (no name clashes).
export const i18n = i18next
const resources = {
  ru: { translation: { ...grafanaRu, ...merge(ru) } },
  en: { translation: merge(en) },
}
if (!i18next.isInitialized) {
  void i18next.use(initReactI18next).init({
    resources,
    lng: getStoredLang(),
    fallbackLng: 'en',
    keySeparator: '.',
    nsSeparator: false,
    returnEmptyString: false,
    interpolation: { escapeValue: false },
  })
} else {
  // Something (in dev: Vite's pre-bundled @grafana/i18n) initialised the shared instance first.
  // Skipping our setup would leave the app on raw keys and en-US — add our bundles to it instead.
  for (const [lng, { translation }] of Object.entries(resources))
    i18next.addResourceBundle(lng, 'translation', translation, true, true)
  i18next.options.fallbackLng = 'en'
  setI18n(i18next)
  void i18next.changeLanguage(getStoredLang())
}
// Point @grafana/i18n's `t`/`Trans` at this instance: without it Grafana's lazy init sees our
// resources, skips itself and leaves its `t` unbound (throws in dev). `getFixedT(null, ns)` reads
// the language on every call, so `changeLanguage` reaches Grafana strings on the next render.
void initPluginTranslations('translation')
