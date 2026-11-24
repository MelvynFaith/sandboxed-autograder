import '@testing-library/jest-dom/vitest'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { applyTheme, getStoredTheme, saveTheme } from './themePreference.js'

afterEach(() => {
  localStorage.clear()
  document.documentElement.removeAttribute('data-theme')
  vi.restoreAllMocks()
})

describe('theme preference', () => {
  it('follows the system until a choice is saved', () => {
    expect(getStoredTheme()).toBe('system')
    applyTheme('system')
    expect(document.documentElement).not.toHaveAttribute('data-theme')
  })

  it('saves and applies light and dark, and clears the choice for the system setting', () => {
    saveTheme('dark')
    expect(localStorage.getItem('theme')).toBe('dark')
    expect(document.documentElement).toHaveAttribute('data-theme', 'dark')
    saveTheme('light')
    expect(getStoredTheme()).toBe('light')
    expect(document.documentElement).toHaveAttribute('data-theme', 'light')
    saveTheme('system')
    expect(localStorage.getItem('theme')).toBeNull()
    expect(document.documentElement).not.toHaveAttribute('data-theme')
  })

  it('ignores a stored value it does not know', () => {
    localStorage.setItem('theme', 'purple')
    expect(getStoredTheme()).toBe('system')
  })

  it('still applies the choice when storage is blocked', () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('blocked')
    })
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('blocked')
    })
    expect(getStoredTheme()).toBe('system')
    expect(() => saveTheme('dark')).not.toThrow()
    expect(document.documentElement).toHaveAttribute('data-theme', 'dark')
  })
})
