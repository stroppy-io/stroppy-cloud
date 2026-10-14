import type { Mode, Palette, Tokens } from './tokens'

// tokens.css is generated from tokens.json by tokensCSS (the test keeps the
// two in sync), so a page without a bundler can link it.

const kebab = (s: string) => s.replace(/[A-Z]/g, (c) => `-${c.toLowerCase()}`)

function palette(p: Palette): string[] {
  return Object.entries(p).map(([k, v]) => `  --st-${kebab(k)}: ${v};`)
}

function mode(m: Mode, t: Tokens): string[] {
  return [
    `  color-scheme: ${m};`,
    ...palette(t.color[m]),
    ...t.series[m].map((v, i) => `  --st-series-${i + 1}: ${v};`),
  ]
}

/** CSS custom properties `--st-*`; dark unless `data-theme="light"`. */
export function tokensCSS(t: Tokens): string {
  const shared = [
    `  --st-font-sans: ${t.font.sans};`,
    `  --st-font-mono: ${t.font.mono};`,
    ...Object.entries(t.font.size).map(([k, v]) => `  --st-font-size-${k}: ${v};`),
    ...Object.entries(t.font.weight).map(([k, v]) => `  --st-font-weight-${k}: ${v};`),
    `  --st-line-height: ${t.font.lineHeight};`,
    `  --st-space: ${t.space};`,
    ...Object.entries(t.radius).map(([k, v]) => `  --st-radius-${k}: ${v};`),
    `  --st-mark: ${t.mark};`,
  ]
  return [
    '/* Generated from tokens.json by css.ts — do not edit. */',
    ':root,',
    '[data-theme="dark"] {',
    ...shared,
    ...mode('dark', t),
    '}',
    '',
    '[data-theme="light"] {',
    ...mode('light', t),
    '}',
    '',
  ].join('\n')
}
