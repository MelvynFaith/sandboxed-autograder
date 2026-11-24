import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

// The theme tokens of index.css must stay readable: text 4.5:1 (WCAG 1.4.3), control borders 3:1
// (WCAG 1.4.11), in light and in dark. The dark values exist twice (system setting and the
// data-theme override); they must not drift apart.
// Vitest runs from the frontend folder.
const css = readFileSync('src/index.css', 'utf8')

function block(opening) {
  const start = css.indexOf(opening)
  expect(start, `block ${opening}`).toBeGreaterThan(-1)
  const end = css.indexOf('\n  }', start) > -1 && opening.startsWith(':root:not')
    ? css.indexOf('\n  }', start)
    : css.indexOf('\n}', start)
  const tokens = {}
  for (const [, name, value] of css.slice(start, end).matchAll(/--([a-z-]+):\s*(#[0-9a-fA-F]{6,8});/g)) {
    tokens[name] = value
  }
  return tokens
}

const light = block(':root {')
const dark = block(':root[data-theme="dark"] {')
const darkMedia = block(':root:not([data-theme="light"]) {')

function rgb(hex) {
  const h = hex.slice(1)
  return [0, 2, 4].map((i) => parseInt(h.slice(i, i + 2), 16))
}

function over(fg, bg) {
  const alpha = fg.length === 9 ? parseInt(fg.slice(7, 9), 16) / 255 : 1
  const f = rgb(fg)
  const b = rgb(bg)
  return f.map((c, i) => Math.round(c * alpha + b[i] * (1 - alpha)))
}

function luminance(channels) {
  const [r, g, b] = channels.map((c) => {
    const s = c / 255
    return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4
  })
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

function contrast(fg, bg, base) {
  const back = bg.length === 9 ? over(bg, base) : rgb(bg)
  const front = fg.length === 9 ? over(fg, bg) : rgb(fg)
  const [hi, lo] = [luminance(front), luminance(back)].sort((a, b) => b - a)
  return (hi + 0.05) / (lo + 0.05)
}

describe.each([
  ['light', light],
  ['dark', dark],
])('%s theme', (_name, t) => {
  it('keeps body text readable on every surface', () => {
    for (const surface of ['canvas', 'surface', 'raised']) {
      expect(contrast(t.ink, t[surface])).toBeGreaterThanOrEqual(4.5)
      expect(contrast(t.muted, t[surface])).toBeGreaterThanOrEqual(4.5)
      expect(contrast(t.link, t[surface])).toBeGreaterThanOrEqual(4.5)
    }
  })

  it('keeps control borders visible', () => {
    expect(contrast(t['line-strong'], t.surface)).toBeGreaterThanOrEqual(3)
    expect(contrast(t['line-strong'], t.canvas)).toBeGreaterThanOrEqual(3)
  })

  it('keeps status text readable on its tint', () => {
    const pairs = [
      ['success', 'success-tint'],
      ['danger', 'danger-tint'],
      ['attention', 'attention-tint'],
      ['done', 'done-tint'],
      ['link', 'accent-tint'],
    ]
    for (const [fg, tint] of pairs) {
      for (const base of ['surface', 'canvas']) {
        expect(contrast(t[fg], t[tint], t[base]), `${fg} on ${tint} over ${base}`).toBeGreaterThanOrEqual(4.5)
      }
    }
  })

  it('keeps white text readable on the filled buttons', () => {
    expect(contrast('#ffffff', t.accent)).toBeGreaterThanOrEqual(4.5)
    expect(contrast('#ffffff', t['accent-hover'])).toBeGreaterThanOrEqual(4.5)
    expect(contrast('#ffffff', t['danger-solid'])).toBeGreaterThanOrEqual(4.5)
    expect(contrast('#ffffff', t['danger-solid-hover'])).toBeGreaterThanOrEqual(4.5)
  })
})

describe('dark theme', () => {
  it('has the same values for the system setting and the override', () => {
    expect(darkMedia).toEqual(dark)
  })
})
