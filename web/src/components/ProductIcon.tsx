import type { IconSize } from '@grafana/ui'
import UilAward from '@iconscout/react-unicons/icons/uil-award'
import UilMedal from '@iconscout/react-unicons/icons/uil-medal'
import UilTrophy from '@iconscout/react-unicons/icons/uil-trophy'
import type { SVGProps } from 'react'

// Icons the Grafana set lacks, from the same Unicons family so they sit next
// to `Icon` without a second style. `Icon` stays first choice; an icon is
// added here, by name, only when Grafana has nothing for it.

const ICONS = {
  trophy: UilTrophy,
  medal: UilMedal,
  award: UilAward,
} as const

export type ProductIconName = keyof typeof ICONS

// The pixel sizes of Grafana's `Icon`, so both read the same.
const SIZES: Record<IconSize, number> = {
  xs: 12,
  sm: 14,
  md: 16,
  lg: 18,
  xl: 24,
  xxl: 36,
  xxxl: 48,
}

export function ProductIcon({
  name,
  size = 'md',
  title,
  ...rest
}: { name: ProductIconName; size?: IconSize; title?: string } & Omit<
  SVGProps<SVGSVGElement>,
  'color' | 'name'
>) {
  const Svg = ICONS[name]
  return (
    <Svg
      size={SIZES[size]}
      color="currentColor"
      role={title ? 'img' : undefined}
      aria-label={title}
      aria-hidden={title ? undefined : true}
      style={{ verticalAlign: 'middle', flexShrink: 0 }}
      {...rest}
    />
  )
}
