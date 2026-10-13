import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import App from './App.jsx'

beforeEach(() => {
  localStorage.clear()
  window.history.pushState({}, '', '/login')
  vi.stubEnv('VITE_USE_MOCK', 'true')
})

afterEach(() => {
  cleanup()
  localStorage.clear()
  vi.unstubAllEnvs()
})

function submitLogin(email, password) {
  fireEvent.change(screen.getByLabelText('Email'), { target: { value: email } })
  fireEvent.change(screen.getByLabelText('Password'), { target: { value: password } })
  fireEvent.click(screen.getByRole('button', { name: 'Masuk' }))
}

describe('authentication routes', () => {
  it('logs in and shows the menu for the authenticated role', async () => {
    render(<App />)

    submitLogin('mahasiswa@example.test', 'password123')

    expect(await screen.findByRole('heading', { name: 'Daftar Assignment' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Riwayat' })).toHaveAttribute('href', '/history')
    expect(screen.getByText('mahasiswa')).toBeInTheDocument()
    expect(localStorage.getItem('token')).toBe('mock-token-mahasiswa')
    expect(JSON.parse(localStorage.getItem('user')).role).toBe('mahasiswa')
  })

  it('shows a useful error when login fails', async () => {
    render(<App />)

    submitLogin('unknown@example.test', 'wrong-password')

    expect(await screen.findByRole('alert')).toHaveTextContent('Email atau password tidak valid.')
    expect(localStorage.getItem('token')).toBeNull()
  })

  it('redirects unauthenticated users to login', async () => {
    window.history.pushState({}, '', '/history')
    render(<App />)

    expect(screen.getByRole('heading', { name: 'Login' })).toBeInTheDocument()
    expect(window.location.pathname).toBe('/login')
    submitLogin('mahasiswa@example.test', 'password123')

    expect(await screen.findByRole('heading', { name: 'Daftar Assignment' })).toBeInTheDocument()
    expect(window.location.pathname).toBe('/assignments')
  })

  it('does not enable mock login unless the flag is true', () => {
    vi.stubEnv('VITE_USE_MOCK', 'false')
    render(<App />)

    expect(screen.queryByText(/Mode mock/)).not.toBeInTheDocument()
  })

  it('shows a different menu for lecturer and admin roles', async () => {
    const { unmount } = render(<App />)
    submitLogin('dosen@example.test', 'password123')

    expect(await screen.findByRole('heading', { name: 'Daftar Assignment' })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Riwayat' })).not.toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Kelola Assignment' })).toHaveAttribute(
      'href',
      '/lecturer/assignments',
    )

    unmount()
    localStorage.clear()
    window.history.pushState({}, '', '/login')
    render(<App />)
    submitLogin('admin@example.test', 'password123')

    expect(await screen.findByRole('heading', { name: 'Daftar Assignment' })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Daftar Assignment' })).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Riwayat' })).not.toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Kelola Assignment' })).toHaveAttribute(
      'href',
      '/lecturer/assignments',
    )
  })

  it('logs out and protects the current route again', async () => {
    render(<App />)
    submitLogin('mahasiswa@example.test', 'password123')

    await screen.findByRole('heading', { name: 'Daftar Assignment' })
    fireEvent.click(screen.getByRole('button', { name: 'Logout' }))

    expect(await screen.findByRole('heading', { name: 'Login' })).toBeInTheDocument()
    expect(localStorage.getItem('token')).toBeNull()
  })
})
