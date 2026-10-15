import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { apiRequest } from '../api/client.js'
import AuthProvider from './AuthContext.jsx'
import { useAuth } from './useAuth.js'

vi.mock('../api/auth.js', () => ({
  login: vi.fn(),
  logout: vi.fn(),
}))

import { logout } from '../api/auth.js'

function AuthHarness() {
  const { user, logout: signOut, sessionNotice } = useAuth()

  return (
    <main>
      <p>{user ? `signed-in:${user.id}` : 'signed-out'}</p>
      {sessionNotice && <p role="status">{sessionNotice}</p>}
      <button onClick={() => apiRequest('/assignments')} type="button">
        Request
      </button>
      <button onClick={() => signOut().catch(() => {})} type="button">
        Logout
      </button>
    </main>
  )
}

function setStoredSession() {
  localStorage.setItem('token', 'expired-token')
  localStorage.setItem('refresh_token', 'refresh-token')
  localStorage.setItem(
    'user',
    JSON.stringify({
      id: 42,
      nama: 'Test User',
      email: 'test@example.test',
      role: 'mahasiswa',
    }),
  )
}

beforeEach(() => {
  localStorage.clear()
  logout.mockReset()
  setStoredSession()
})

afterEach(() => {
  cleanup()
  localStorage.clear()
  vi.unstubAllGlobals()
})

describe('AuthProvider session recovery', () => {
  it('normalizes stored numeric user ids and clears React auth state after refresh fails', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({ status: 401, ok: false })
      .mockResolvedValueOnce({ status: 401, ok: false })
    vi.stubGlobal('fetch', fetchMock)
    render(
      <AuthProvider>
        <AuthHarness />
      </AuthProvider>,
    )

    expect(screen.getByText('signed-in:42')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Request' }))

    expect(await screen.findByText('signed-out')).toBeInTheDocument()
    expect(screen.getByRole('status')).toHaveTextContent('Sesi berakhir. Silakan masuk kembali.')
    expect(localStorage.getItem('token')).toBeNull()
    expect(localStorage.getItem('refresh_token')).toBeNull()
  })

  it('clears the local session and shows a notice when server logout fails', async () => {
    logout.mockRejectedValue(new Error('Server tidak tersedia.'))
    render(
      <AuthProvider>
        <AuthHarness />
      </AuthProvider>,
    )

    fireEvent.click(screen.getByRole('button', { name: 'Logout' }))

    expect(await screen.findByText('signed-out')).toBeInTheDocument()
    expect(screen.getByRole('status')).toHaveTextContent(
      'Sesi lokal sudah dihapus. Logout server gagal: Server tidak tersedia.',
    )
    expect(localStorage.getItem('token')).toBeNull()
    expect(localStorage.getItem('refresh_token')).toBeNull()
  })
})
