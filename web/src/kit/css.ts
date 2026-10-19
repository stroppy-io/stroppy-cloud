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

/**
 * CSS custom properties `--st-*` with three ways to pick the palette, all
 * plain CSS:
 *
 * - no `data-theme` (or `data-theme="dark"`): dark;
 * - `data-theme="light"`: light;
 * - `data-theme="system"`: the reader's `prefers-color-scheme`.
 *
 * The attribute works on any element, nearest one wins, and an explicit
 * light or dark beats the system preference. Print is light whatever the
 * screen shows: paper is white.
 */
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
  const indent = (lines: string[]) => lines.map((l) => `  ${l}`)
  return [
    '/* Generated from tokens.json by css.ts — do not edit.',
    '   Theme: data-theme="dark" (default) | "light" | "system" on any element;',
    '   print is always light. */',
    ':root {',
    ...shared,
    '}',
    '',
    ':root,',
    '[data-theme="dark"] {',
    ...mode('dark', t),
    '}',
    '',
    '[data-theme="light"] {',
    ...mode('light', t),
    '}',
    '',
    '@media (prefers-color-scheme: light) {',
    '  [data-theme="system"] {',
    ...indent(mode('light', t)),
    '  }',
    '}',
    '',
    '@media (prefers-color-scheme: dark) {',
    '  [data-theme="system"] {',
    ...indent(mode('dark', t)),
    '  }',
    '}',
    '',
    '@media print {',
    '  :root,',
    '  [data-theme] {',
    ...indent(mode('light', t)),
    '  }',
    '}',
    '',
  ].join('\n')
}
