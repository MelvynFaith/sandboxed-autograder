import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import ThemeSetting from './ThemeSetting.jsx'

afterEach(() => {
  cleanup()
  localStorage.clear()
  document.documentElement.removeAttribute('data-theme')
})

describe('ThemeSetting', () => {
  it('starts on the system setting and switches the theme when a choice is picked', () => {
    render(<ThemeSetting />)
    expect(screen.getByRole('radio', { name: /Ikuti sistem/ })).toBeChecked()

    fireEvent.click(screen.getByRole('radio', { name: /Gelap/ }))
    expect(screen.getByRole('radio', { name: /Gelap/ })).toBeChecked()
    expect(document.documentElement).toHaveAttribute('data-theme', 'dark')
    expect(localStorage.getItem('theme')).toBe('dark')

    fireEvent.click(screen.getByRole('radio', { name: /Ikuti sistem/ }))
    expect(document.documentElement).not.toHaveAttribute('data-theme')
  })

  it('shows a saved choice', () => {
    localStorage.setItem('theme', 'light')
    render(<ThemeSetting />)
    expect(screen.getByRole('radio', { name: /Terang/ })).toBeChecked()
  })
})
