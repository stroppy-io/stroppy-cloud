import { useTheme2 } from '@grafana/ui'
import { BAND, BODY, RAYS } from '@/kit/logo'
import { tokens } from '@/kit/tokens'

// The Stroppy mark (geometry and orange from the kit), drawn inline so it
// takes the theme: the body follows the surrounding text colour, the rays and
// the middle band are orange: a database under stress. An <img> could not take
// the text colour; public/org-logo.svg is the favicon.

/** The orange of the mark. Same in both themes. */
export const MARK_ORANGE = tokens.mark

// Compact mark for small sizes (headers, ≤ COMPACT_MAX px): the same rays,
// thickened by a stroke of their own colour (hairlines vanish there). The cylinder is unchanged.
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
 * (default: secondary text, so the orange carries the mark and the name next
 * to it stays brighter). Sizes up to 40px draw the compact mark.
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
  const accent = MARK_ORANGE
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
      <g fill={color ?? theme.colors.text.secondary}>
        {BODY.map((d) => (
          <path key={d} d={d} />
        ))}
      </g>
    </svg>
  )
}
