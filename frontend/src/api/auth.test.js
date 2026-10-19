import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { login, logout, register } from './auth.js'
import { apiRequest } from './client.js'

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
      name: 'Nama Pengguna',
      email: 'user@example.test',
      role: 'STUDENT',
    }
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ access_token: 'jwt-token', refresh_token: 'refresh-token', user }),
    })

    vi.stubGlobal('fetch', fetchMock)

    await expect(login('user@example.test', 'secret')).resolves.toEqual({
      token: 'jwt-token',
      refreshToken: 'refresh-token',
      user: { id: 'user-1', nama: 'Nama Pengguna', email: 'user@example.test', role: 'mahasiswa' },
    })
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/auth/login',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ email: 'user@example.test', password: 'secret' }),
      }),
    )
  })

  it('normalizes a numeric user id from the API response', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        access_token: 'jwt-token',
        refresh_token: 'refresh-token',
        user: {
          id: 42,
          name: 'Nama Pengguna',
          email: 'user@example.test',
          role: 'STUDENT',
        },
      }),
    }))

    await expect(login('user@example.test', 'secret')).resolves.toMatchObject({
      user: { id: '42', role: 'mahasiswa' },
    })
  })

  describe('registration API', () => {
    it('posts the assumed registration body and reads the resulting student user', async () => {
      const user = {
        id: 'user-2',
        name: 'Siswa Baru',
        email: 'baru@example.test',
        role: 'STUDENT',
      }
      const fetchMock = vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({ user }),
      })
      vi.stubGlobal('fetch', fetchMock)

      await expect(register('Siswa Baru', 'baru@example.test', 'secret')).resolves.toEqual(user)
      expect(fetchMock).toHaveBeenCalledWith(
        '/api/v1/auth/register',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ name: 'Siswa Baru', email: 'baru@example.test', password: 'secret' }),
        }),
      )
    })

    it('displays the API message when registration rejects a duplicate email', async () => {
      vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
        ok: false,
        status: 409,
        json: async () => ({ error: { code: 'EMAIL_EXISTS', message: 'Email sudah terdaftar.' } }),
      }))

      await expect(register('Siswa', 'duplikat@example.test', 'secret')).rejects.toThrow(
        'Email sudah terdaftar.',
      )
    })
  })

  it('revokes the current refresh token through the logout endpoint', async () => {
    const fetchMock = vi.fn().mockResolvedValue({ ok: true, status: 204 })
    vi.stubGlobal('fetch', fetchMock)

    await expect(logout('refresh-token')).resolves.toBeUndefined()
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/auth/logout',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ refresh_token: 'refresh-token' }),
      }),
    )
  })

  it('includes the saved JWT in the Authorization header for API requests', async () => {
    localStorage.setItem('token', 'jwt-token')
    const fetchMock = vi.fn().mockResolvedValue({ ok: true })
    vi.stubGlobal('fetch', fetchMock)

    await apiRequest('/assignments')

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

  it('rejects role names that are not defined by the API', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({
          access_token: 'jwt-token',
          refresh_token: 'refresh-token',
          user: {
            id: 'user-1',
            name: 'Pengguna',
            email: 'user@example.test',
            role: 'toString',
          },
        }),
      }),
    )

    await expect(login('user@example.test', 'secret')).rejects.toThrow(
      'Respons login tidak sesuai format yang diharapkan.',
    )
  })
})
