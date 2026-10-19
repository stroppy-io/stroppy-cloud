import { readFileSync, writeFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { align, formatElapsed, hatchDefs, hatchPath } from './chart'
import { tokensCSS } from './css'
import { tokens } from './tokens'

describe('kit chart', () => {
  it('keeps a missing observation a gap and steps over alignment slots', () => {
    const data = align([
      {
        title: 'a',
        points: [
          [10, 1],
          [20, null],
          [30, 3],
        ],
      },
      {
        title: 'b',
        points: [
          [15, 5],
          [30, 6],
        ],
      },
    ])
    expect(data[0]).toEqual([10, 15, 20, 30])
    // a: the page's null at 20 stays null (a gap); 15 exists only for b → undefined.
    expect(data[1]).toEqual([1, undefined, null, 3])
    expect(data[2]).toEqual([undefined, 5, undefined, 6])
  })

  it('counts elapsed time from the origin, milliseconds or seconds', () => {
    const data = align([{ title: 'a', points: [[1_000_000_060_000, 1]] }], 1_000_000_000)
    expect(data[0]).toEqual([60])
    expect(formatElapsed(75)).toBe('1:15')
    expect(formatElapsed(3725)).toBe('1:02:05')
    expect(formatElapsed(-30)).toBe('−0:30')
  })
})

describe('kit themes', () => {
  const css = tokensCSS(tokens)
  it('lets the reader pick light, dark or the system scheme with CSS alone', () => {
    expect(css).toContain('[data-theme="light"] {')
    expect(css).toMatch(/@media \(prefers-color-scheme: light\) \{\n {2}\[data-theme="system"\]/)
    expect(css).toMatch(/@media \(prefers-color-scheme: dark\) \{\n {2}\[data-theme="system"\]/)
  })
  it('prints light whatever the screen shows', () => {
    const print = css.slice(css.indexOf('@media print'))
    expect(print).toContain(`--st-canvas: ${tokens.color.light.canvas};`)
    expect(print).toContain('[data-theme] {')
  })
})

describe('kit hatches', () => {
  it('series 1 is solid, every other series has its own hatch', () => {
    expect(hatchPath(1)).toBeNull()
    const marks = [2, 3, 4, 5, 6, 7, 8].map(hatchPath)
    expect(new Set(marks).size).toBe(7)
    expect(hatchPath(9)).toBeNull()
  })
  // hatch.svg is what a static page pastes; KIT_UPDATE=1 regenerates it.
  it('hatch.svg matches the hatches the live chart draws', () => {
    const path = fileURLToPath(new URL('./hatch.svg', import.meta.url))
    const want = `${hatchDefs()}\n`
    if (process.env.KIT_UPDATE) writeFileSync(path, want)
    expect(readFileSync(path, 'utf8')).toBe(want)
  })
})
