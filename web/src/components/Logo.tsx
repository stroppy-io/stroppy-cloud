import { useTheme2 } from '@grafana/ui'

// The Stroppy mark, drawn inline so it takes the theme: the body follows the
// surrounding text colour, the rays and the middle band are the app's accent
// (primary). An <img> of public/org-logo.svg could not do either; that file
// stays the source and the favicon.

// Full mark — the geometry of public/org-logo.svg.
export const RAYS = [
  'M32 24.5 42 29.5 36 29 30.5 26.5Z',
  'M44 18 48.5 18.5 50 27 46.8 23Z',
  'M62 18 65.5 18 65 27 62.5 24Z',
  'M80.5 18 84 18.5 78 28 77 24Z',
  'M95.5 24 96 27 86 30 91 26Z',
]
export const BAND =
  'M31 81 C37 76.5 49 74 64 74 C79 74 91 77.2 97 81.5 L97 85.2 C90 89.6 78 92 64 92 C49 92 37 88.7 31 84.5Z'
export const BODY = [
  'M31 42 C34 37.5 47 34 63.5 34 C80 34 92 37.5 97 42.5 C93 47 80 50.5 64 50.5 C48 50.5 36 47.3 31 42Z',
  'M30.5 49 C40 54.3 51 57.2 64 57.2 C78 57.2 89 54.3 97.5 49 L97.5 66.7 C93 72 80 75.1 64 75.1 C48 75.1 35 72 30.5 66.7Z',
  'M31 86.5 C39 91 50 93.1 64 93.1 C78 93.1 90 90.8 97 86.5 L97 100.8 C97 106.3 82.5 110.8 64 110.8 C45.5 110.8 31 106.3 31 100.8Z',
]

// Compact mark for small sizes (headers, ≤ COMPACT_MAX px): the same rays,
// thickened by a stroke of their own colour (hairlines vanish there), in the
// brighter accent tone. The cylinder is unchanged.
const COMPACT_MAX = 40
const COMPACT_STROKE = 3.5

// Boxes are centred on the cylinder (y 34–110.8), not on the whole drawing:
// the rays on top carry little weight, so a mark centred by its bounds sits
// visibly low next to text. Each box runs from the rays' top down to the same
// distance below the cylinder's centre.
const BOX_FULL = { x: 30, y: 17.5, w: 68, h: 109.8 }
const BOX_COMPACT = { x: 28, y: 16, w: 72, h: 112.8 }

/**
 * The Stroppy logo, `size` px tall; `color` overrides the body colour
 * (default: theme text). Sizes up to 40px draw the compact mark.
 */
export function Logo({
  size = 24,
  color,
  className,
}: {
  size?: number
  color?: string
  className?: string
}) {
  const theme = useTheme2()
  const compact = size <= COMPACT_MAX
  const box = compact ? BOX_COMPACT : BOX_FULL
  const accent = compact ? theme.colors.primary.text : theme.colors.primary.main
  return (
    <svg
      className={className}
      width={(size * box.w) / box.h}
      height={size}
      viewBox={`${box.x} ${box.y} ${box.w} ${box.h}`}
      // Never squeezed by a flex row: it keeps its proportions.
      style={{ flexShrink: 0 }}
      aria-hidden="true"
      focusable="false"
    >
      <g
        fill={accent}
        stroke={compact ? accent : undefined}
        strokeWidth={compact ? COMPACT_STROKE : undefined}
        strokeLinejoin="round"
      >
        {RAYS.map((d) => (
          <path key={d} d={d} />
        ))}
      </g>
      <path d={BAND} fill={accent} />
      <g fill={color ?? theme.colors.text.primary}>
        {BODY.map((d) => (
          <path key={d} d={d} />
        ))}
      </g>
    </svg>
  )
}
