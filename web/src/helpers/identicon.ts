export function identicon(seed: string): string {
  // Deterministic 5×5 symmetric pattern; enough for a visual identity without an upload.
  let h = 0
  for (const ch of seed) h = (h * 31 + ch.charCodeAt(0)) >>> 0
  const cells: string[] = []
  for (let y = 0; y < 5; y++)
    for (let x = 0; x < 3; x++) {
      h = (h * 1664525 + 1013904223) >>> 0
      if (h & 0x8000) {
        cells.push(`<rect x="${x}" y="${y}" width="1" height="1"/>`)
        if (x < 2) cells.push(`<rect x="${4 - x}" y="${y}" width="1" height="1"/>`)
      }
    }
  const hue = h % 360
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="-0.5 -0.5 6 6" shape-rendering="crispEdges"><g fill="hsl(${hue} 55% 55%)">${cells.join('')}</g></svg>`
  return `data:image/svg+xml;utf8,${encodeURIComponent(svg)}`
}
