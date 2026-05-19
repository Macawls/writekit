const SAT = 0.72
const LIGHT = 0.50
const STANDALONE_HSL = { h: 0, s: 0, l: 0.47 }

function hashStr(s: string): number {
  let h = 2166136261
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i)
    h = Math.imul(h, 16777619)
  }
  return h >>> 0
}

export function collectionHsl(id: string | null | undefined): { h: number; s: number; l: number } {
  if (!id) return STANDALONE_HSL
  return { h: (hashStr(id) % 360) / 360, s: SAT, l: LIGHT }
}

function hslToRgb(h: number, s: number, l: number): [number, number, number] {
  if (s === 0) return [l, l, l]
  const q = l < 0.5 ? l * (1 + s) : l + s - l * s
  const p = 2 * l - q
  const hue2rgb = (t: number) => {
    if (t < 0) t += 1
    if (t > 1) t -= 1
    if (t < 1 / 6) return p + (q - p) * 6 * t
    if (t < 1 / 2) return q
    if (t < 2 / 3) return p + (q - p) * (2 / 3 - t) * 6
    return p
  }
  return [hue2rgb(h + 1 / 3), hue2rgb(h), hue2rgb(h - 1 / 3)]
}

export function collectionColor(id: string | null | undefined): string {
  const { h, s, l } = collectionHsl(id)
  const [r, g, b] = hslToRgb(h, s, l)
  const to255 = (v: number) => Math.round(v * 255).toString(16).padStart(2, '0')
  return `#${to255(r)}${to255(g)}${to255(b)}`
}

export const STANDALONE_COLOR = collectionColor(null)
