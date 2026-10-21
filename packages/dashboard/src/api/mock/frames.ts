// Stand-in camera frames for the mock: the sorter's platform seen from above,
// the item on it and the detector's box, drawn as SVG.

export type Item = 'mandarin' | 'cup' | 'kiwi' | 'ball' | 'hand'

export type BBox = [x1: number, y1: number, x2: number, y2: number]

function itemShape(item: Item, [x1, y1, x2, y2]: BBox): string {
  const cx = (x1 + x2) / 2
  const cy = (y1 + y2) / 2
  const r = Math.min(x2 - x1, y2 - y1) / 2
  switch (item) {
    case 'mandarin':
      return `
        <circle cx="${cx}" cy="${cy}" r="${r * 0.9}" fill="#f28c28"/>
        <circle cx="${cx - r * 0.3}" cy="${cy - r * 0.3}" r="${r * 0.25}" fill="#f7b267" opacity=".7"/>
        <ellipse cx="${cx + r * 0.1}" cy="${cy - r * 0.85}" rx="${r * 0.25}" ry="${r * 0.12}" fill="#3f8f3f"/>`
    case 'ball':
      return `
        <circle cx="${cx}" cy="${cy}" r="${r * 0.9}" fill="#d7e34a"/>
        <path d="M ${cx - r * 0.85} ${cy - r * 0.3} Q ${cx} ${cy + r * 0.4} ${cx + r * 0.85} ${cy - r * 0.3}" stroke="#f5f7e8" stroke-width="${r * 0.12}" fill="none"/>`
    case 'kiwi':
      return `
        <ellipse cx="${cx}" cy="${cy}" rx="${r}" ry="${r * 0.78}" fill="#8a6a43"/>
        <ellipse cx="${cx - r * 0.25}" cy="${cy - r * 0.2}" rx="${r * 0.35}" ry="${r * 0.2}" fill="#a3825a" opacity=".6"/>`
    case 'cup':
      return `
        <circle cx="${cx}" cy="${cy}" r="${r * 0.95}" fill="#f4f6f8" stroke="#c4ccd6" stroke-width="6"/>
        <circle cx="${cx}" cy="${cy}" r="${r * 0.7}" fill="#e3e8ee"/>`
    case 'hand':
      return `
        <g fill="#d9a07b">
          <rect x="${x1}" y="${cy - r * 0.45}" width="${(x2 - x1) * 0.62}" height="${r * 0.9}" rx="${r * 0.35}"/>
          <rect x="${x1 + (x2 - x1) * 0.55}" y="${cy - r * 0.62}" width="${(x2 - x1) * 0.4}" height="${r * 0.22}" rx="${r * 0.11}"/>
          <rect x="${x1 + (x2 - x1) * 0.55}" y="${cy - r * 0.32}" width="${(x2 - x1) * 0.45}" height="${r * 0.22}" rx="${r * 0.11}"/>
          <rect x="${x1 + (x2 - x1) * 0.55}" y="${cy - r * 0.02}" width="${(x2 - x1) * 0.42}" height="${r * 0.22}" rx="${r * 0.11}"/>
          <rect x="${x1 + (x2 - x1) * 0.5}" y="${cy + r * 0.28}" width="${(x2 - x1) * 0.32}" height="${r * 0.22}" rx="${r * 0.11}"/>
        </g>`
  }
}

/** A 640×480 frame as a data URL, like the sorter's JPEG would be served. */
export function frameDataUrl(item: Item, bbox: BBox, label: string): string {
  const [x1, y1, x2, y2] = bbox
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="640" height="480" viewBox="0 0 640 480">
    <defs>
      <radialGradient id="light" cx="50%" cy="45%" r="70%">
        <stop offset="0" stop-color="#3a3d40"/><stop offset="1" stop-color="#151617"/>
      </radialGradient>
    </defs>
    <rect width="640" height="480" fill="url(#light)"/>
    <rect x="70" y="50" width="500" height="380" rx="18" fill="#2a2c2f" stroke="#45484c" stroke-width="3"/>
    ${itemShape(item, bbox)}
    <rect x="${x1}" y="${y1}" width="${x2 - x1}" height="${y2 - y1}" fill="none" stroke="#ffffff" stroke-width="2"/>
    <rect x="${x1}" y="${y1 - 26}" width="${label.length * 11 + 16}" height="26" fill="#ffffff"/>
    <text x="${x1 + 8}" y="${y1 - 8}" font-family="ui-monospace, monospace" font-size="15" fill="#000">${label}</text>
  </svg>`
  return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`
}
