import { type RefObject, useLayoutEffect, useRef, useState } from 'react'

// Distance from the top of the viewport to an element (page scrolled to top), re-measured on
// resize and when anything above it changes size. Lets a panel take `calc(100vh - top)` so the
// panel scrolls instead of the page.
export function useOffsetTop<T extends HTMLElement = HTMLDivElement>(): [
  RefObject<T | null>,
  number,
] {
  const ref = useRef<T | null>(null)
  const [top, setTop] = useState(0)
  useLayoutEffect(() => {
    const el = ref.current
    if (!el) return
    const update = () => {
      const next = Math.round(el.getBoundingClientRect().top + window.scrollY)
      setTop((prev) => (prev === next ? prev : next))
    }
    update()
    const ro = new ResizeObserver(update)
    ro.observe(document.body)
    window.addEventListener('resize', update)
    return () => {
      ro.disconnect()
      window.removeEventListener('resize', update)
    }
  }, [])
  return [ref, top]
}
