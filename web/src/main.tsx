import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { i18nReady } from './lib/i18n'

// @grafana/ui calls its `t()` at module scope; the app is imported only once
// the shared i18n instance is ready (the dev build throws otherwise).
await i18nReady
const { App } = await import('./App')

const root = document.getElementById('root')
if (!root) throw new Error('#root missing')
createRoot(root).render(
  <StrictMode>
    <App />
  </StrictMode>
)
