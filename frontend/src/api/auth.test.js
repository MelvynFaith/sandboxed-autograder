import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { login } from './auth.js'

beforeEach(() => {
  localStorage.clear()
  vi.stubEnv('VITE_USE_MOCK', 'false')
})

afterEach(() => {
  localStorage.clear()
  vi.unstubAllEnvs()
  vi.unstubAllGlobals()
})

describe('login API', () => {
  it('posts credentials to the login endpoint and reads the documented assumption', async () => {
    const user = {
      id: 'user-1',
      nama: 'Nama Pengguna',
      email: 'user@example.test',
      role: 'mahasiswa',
    }
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ token: 'jwt-token', user }),
    })
    vi.stubGlobal('fetch', fetchMock)

    await expect(login('user@example.test', 'secret')).resolves.toEqual({
      token: 'jwt-token',
      user,
    })
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/auth/login',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ email: 'user@example.test', password: 'secret' }),
      }),
    )
  })

  it('includes the saved JWT in the Authorization header for API requests', async () => {
    localStorage.setItem('token', 'jwt-token')
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        token: 'next-token',
        user: {
          id: 'user-1',
          nama: 'Nama Pengguna',
          email: 'user@example.test',
          role: 'mahasiswa',
        },
      }),
    })
    vi.stubGlobal('fetch', fetchMock)

    await login('user@example.test', 'secret')

    expect(fetchMock.mock.calls[0][1].headers.get('Authorization')).toBe('Bearer jwt-token')
  })

  it('rejects responses that do not match the assumed login response shape', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({ ok: true, json: async () => ({ access_token: 'jwt-token' }) }),
    )

    await expect(login('user@example.test', 'secret')).rejects.toThrow(
      'Respons login tidak sesuai format yang diharapkan.',
    )
  })
})
