import { type RefObject, useEffect, useRef, useState } from 'react'

export interface Measured {
  width: number
  height: number
}

// Content-box size of a DOM element, updated through ResizeObserver. Grafana viz components
// (`TimeSeries`, `BigValue`) need explicit pixel sizes, so panels measure their container.
export function useMeasure<T extends HTMLElement = HTMLDivElement>(): [
  RefObject<T | null>,
  Measured,
] {
  const ref = useRef<T | null>(null)
  const [size, setSize] = useState<Measured>({ width: 0, height: 0 })
  useEffect(() => {
    const el = ref.current
    if (!el) return
    const update = () => {
      const r = el.getBoundingClientRect()
      const next = { width: Math.floor(r.width), height: Math.floor(r.height) }
      setSize((prev) => (prev.width === next.width && prev.height === next.height ? prev : next))
    }
    update()
    const ro = new ResizeObserver(update)
    ro.observe(el)
    return () => ro.disconnect()
  }, [])
  return [ref, size]
}
