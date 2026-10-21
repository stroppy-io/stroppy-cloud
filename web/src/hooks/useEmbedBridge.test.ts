import { describe, expect, it } from 'vitest'
import { themeCommand } from './useEmbedBridge'

describe('embed bridge', () => {
  const origins = ['https://stroppy.io', 'http://localhost:3000']
  it('takes a theme command from an allowed page', () => {
    const data = { type: 'stroppy-embed:theme', theme: 'light' }
    expect(themeCommand({ origin: 'https://stroppy.io', data }, origins)).toBe('light')
    expect(
      themeCommand({ origin: 'http://localhost:3000', data: { ...data, theme: 'dark' } }, origins)
    ).toBe('dark')
  })
  it('ignores other origins, other messages and unknown themes', () => {
    const data = { type: 'stroppy-embed:theme', theme: 'light' }
    expect(themeCommand({ origin: 'https://evil.example', data }, origins)).toBeUndefined()
    expect(
      themeCommand(
        { origin: 'https://stroppy.io', data: { type: 'other', theme: 'light' } },
        origins
      )
    ).toBeUndefined()
    expect(
      themeCommand({ origin: 'https://stroppy.io', data: { ...data, theme: 'pink' } }, origins)
    ).toBeUndefined()
    expect(themeCommand({ origin: 'https://stroppy.io', data: null }, origins)).toBeUndefined()
    expect(themeCommand({ origin: 'https://stroppy.io', data }, [])).toBeUndefined()
  })
})
