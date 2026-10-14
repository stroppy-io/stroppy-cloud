import { readFileSync, writeFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { expect, test } from 'vitest'
import { tokensCSS } from './css'
import { tokens } from './tokens'

// tokens.css is what a page without a bundler links. Run with
// KIT_UPDATE=1 after changing tokens.json to regenerate it.
test('tokens.css matches tokens.json', () => {
  const path = fileURLToPath(new URL('./tokens.css', import.meta.url))
  const want = tokensCSS(tokens)
  if (process.env.KIT_UPDATE) writeFileSync(path, want)
  expect(readFileSync(path, 'utf8')).toBe(want)
})
