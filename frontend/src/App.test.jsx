import '@testing-library/jest-dom/vitest'
import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import App from './App.jsx'

describe('App', () => {
  it('renders the placeholder routes and navigation', () => {
    render(<App />)

    expect(screen.getByRole('heading', { name: 'Login' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Daftar Assignment' })).toHaveAttribute(
      'href',
      '/assignments',
    )
    expect(screen.getByRole('link', { name: 'Riwayat' })).toHaveAttribute('href', '/history')

    fireEvent.click(screen.getByRole('link', { name: 'Daftar Assignment' }))
    expect(screen.getByRole('heading', { name: 'Daftar Assignment' })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('link', { name: 'Riwayat' }))
    expect(screen.getByRole('heading', { name: 'Riwayat' })).toBeInTheDocument()
  })
})
