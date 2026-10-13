// @iconscout/react-unicons ships no types: each icon is a component taking
// a size and a color, the rest go to the <svg>.
declare module '@iconscout/react-unicons/icons/*' {
  import type { FC, SVGProps } from 'react'

  const Icon: FC<
    Omit<SVGProps<SVGSVGElement>, 'color'> & { size?: number | string; color?: string }
  >
  export default Icon
}
