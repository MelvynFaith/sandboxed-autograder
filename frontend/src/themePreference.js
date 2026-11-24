// The appearance setting: follow the system, or force light or dark. It is a per-browser
// convenience, so storage is read and written defensively (private windows and blocked site data
// throw); without it the page follows the system.
const KEY = 'theme'

export const THEMES = ['system', 'light', 'dark']

export function getStoredTheme() {
  try {
    const value = localStorage.getItem(KEY)
    return THEMES.includes(value) ? value : 'system'
  } catch {
    return 'system'
  }
}

// "system" removes the attribute, so the prefers-color-scheme rule of index.css decides.
export function applyTheme(theme) {
  const root = document.documentElement
  if (theme === 'light' || theme === 'dark') {
    root.setAttribute('data-theme', theme)
  } else {
    root.removeAttribute('data-theme')
  }
}

export function saveTheme(theme) {
  try {
    if (theme === 'system') {
      localStorage.removeItem(KEY)
    } else {
      localStorage.setItem(KEY, theme)
    }
  } catch {
    // The choice still applies to this page view.
  }
  applyTheme(theme)
}
