import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { getProfile, updatePassword, updateProfileName } from './profile.js'

const user = {
  id: 'user-1',
  name: 'Nama',
  email: 'user@example.test',
  role: 'STUDENT',
}

beforeEach(() => {
  localStorage.clear()
  vi.stubEnv('VITE_USE_MOCK', 'false')
})

afterEach(() => {
  localStorage.clear()
  vi.unstubAllEnvs()
  vi.unstubAllGlobals()
})

describe('profile API', () => {
  it('reads and updates the current user at the documented endpoints', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({ ok: true, json: async () => ({ user }) })
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({ user: { ...user, name: 'Nama Baru' } }),
      })
    vi.stubGlobal('fetch', fetchMock)

    await expect(getProfile({ id: 'user-1', nama: 'Nama', email: user.email, role: 'mahasiswa' }))
      .resolves.toEqual(user)
    await expect(updateProfileName('Nama Baru', { id: 'user-1' }))
      .resolves.toEqual({ ...user, name: 'Nama Baru' })

    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/users/me')
    expect(fetchMock.mock.calls[0][1].method).toBeUndefined()
    expect(fetchMock.mock.calls[1][0]).toBe('/api/v1/users/me')
    expect(fetchMock.mock.calls[1][1]).toEqual(expect.objectContaining({
      method: 'PUT',
      body: JSON.stringify({ name: 'Nama Baru' }),
    }))
  })

  it('verifies the old password through the password endpoint', async () => {
    const fetchMock = vi.fn().mockResolvedValue({ ok: true })
    vi.stubGlobal('fetch', fetchMock)

    await expect(updatePassword('old-secret', 'new-secret', { email: user.email })).resolves.toBeUndefined()
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/users/me/password',
      expect.objectContaining({
        method: 'PUT',
        body: JSON.stringify({ current_password: 'old-secret', new_password: 'new-secret' }),
      }),
    )
  })
})
