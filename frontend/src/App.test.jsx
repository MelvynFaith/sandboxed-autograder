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
    expect(localStorage.getItem('token')).toBe('mock-access-token-STUDENT')
    expect(localStorage.getItem('refresh_token')).toBe('mock-refresh-token-STUDENT')
    expect(JSON.parse(localStorage.getItem('user')).role).toBe('mahasiswa')
  })

  it('shows a useful error when login fails', async () => {
    render(<App />)

    submitLogin('unknown@example.test', 'wrong-password')

    expect(await screen.findByRole('alert')).toHaveTextContent('Email atau password tidak valid.')
    expect(localStorage.getItem('token')).toBeNull()
  })

  it('registers a student and redirects to login', async () => {
    render(<App />)
    fireEvent.click(screen.getByRole('link', { name: 'Register' }))
    fireEvent.change(screen.getByLabelText('Nama'), { target: { value: 'Siswa Baru' } })
    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'baru@example.test' } })
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'secret123' } })
    fireEvent.click(screen.getByRole('button', { name: 'Daftar' }))

    expect(await screen.findByRole('heading', { name: 'Login' })).toBeInTheDocument()
    expect(localStorage.getItem('token')).toBeNull()
  })

  it('shows a clear message when registration email is already used', async () => {
    render(<App />)
    fireEvent.click(screen.getByRole('link', { name: 'Register' }))
    fireEvent.change(screen.getByLabelText('Nama'), { target: { value: 'Siswa Baru' } })
    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'mahasiswa@example.test' } })
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'secret123' } })
    fireEvent.click(screen.getByRole('button', { name: 'Daftar' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Email sudah terdaftar.')
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
    expect(screen.getByRole('link', { name: 'Profil' })).toHaveAttribute('href', '/profile')
    expect(screen.getByRole('link', { name: 'Kelola Assignment' })).toHaveAttribute(
      'href',
      '/lecturer/assignments',
    )

    unmount()
    localStorage.clear()
    window.history.pushState({}, '', '/login')
    render(<App />)
    submitLogin('admin@example.test', 'password123')

    expect(await screen.findByRole('heading', { name: 'Administrasi' })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Riwayat' })).not.toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Profil' })).toHaveAttribute('href', '/profile')
    expect(screen.getByRole('link', { name: 'Kelola Assignment' })).toHaveAttribute(
      'href',
      '/lecturer/assignments',
    )
    expect(screen.getByRole('link', { name: 'Administrasi' })).toHaveAttribute('href', '/admin')
  })

  it('logs out and protects the current route again', async () => {
    render(<App />)
    submitLogin('mahasiswa@example.test', 'password123')

    await screen.findByRole('heading', { name: 'Daftar Assignment' })
    fireEvent.click(screen.getByRole('button', { name: 'Logout' }))

    expect(await screen.findByRole('heading', { name: 'Login' })).toBeInTheDocument()
    expect(localStorage.getItem('token')).toBeNull()
  })

  it('updates the current user name on the profile page', async () => {
    render(<App />)
    submitLogin('mahasiswa@example.test', 'password123')
    await screen.findByRole('heading', { name: 'Daftar Assignment' })
    fireEvent.click(screen.getByRole('link', { name: 'Profil' }))

    const nameInput = await screen.findByLabelText('Nama tampilan')
    expect(screen.getByText('Mahasiswa Demo')).toBeInTheDocument()
    fireEvent.change(nameInput, { target: { value: 'Nama Baru' } })
    fireEvent.click(screen.getByRole('button', { name: 'Simpan nama' }))

    expect(await screen.findByText('Nama berhasil diperbarui.')).toBeInTheDocument()
    expect(screen.getByText('Nama Baru')).toBeInTheDocument()
    expect(JSON.parse(localStorage.getItem('user')).nama).toBe('Nama Baru')
  })

  it('clears the session after a password change and redirects to login', async () => {
    render(<App />)
    submitLogin('mahasiswa@example.test', 'password123')
    await screen.findByRole('heading', { name: 'Daftar Assignment' })
    fireEvent.click(screen.getByRole('link', { name: 'Profil' }))
    await screen.findByRole('heading', { name: 'Profil' })
    fireEvent.change(screen.getByLabelText('Kata sandi lama'), { target: { value: 'password123' } })
    fireEvent.change(screen.getByLabelText('Kata sandi baru'), { target: { value: 'new-password' } })
    fireEvent.click(screen.getByRole('button', { name: 'Ganti kata sandi' }))

    expect(await screen.findByRole('heading', { name: 'Login' })).toBeInTheDocument()
    expect(screen.getByRole('status')).toHaveTextContent('Silakan masuk kembali')
    expect(localStorage.getItem('token')).toBeNull()
    expect(localStorage.getItem('refresh_token')).toBeNull()
    expect(localStorage.getItem('user')).toBeNull()

    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'mahasiswa@example.test' } })
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'new-password' } })
    fireEvent.click(screen.getByRole('button', { name: 'Masuk' }))
    expect(await screen.findByRole('heading', { name: 'Daftar Assignment' })).toBeInTheDocument()
  })

  it('redirects an unauthenticated profile route to login', () => {
    window.history.pushState({}, '', '/profile')
    render(<App />)

    expect(screen.getByRole('heading', { name: 'Login' })).toBeInTheDocument()
    expect(window.location.pathname).toBe('/login')
  })
})
