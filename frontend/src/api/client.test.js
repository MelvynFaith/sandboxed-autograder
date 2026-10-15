import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { apiRequest, throwApiError } from './client.js'

beforeEach(() => {
  localStorage.clear()
})

afterEach(() => {
  localStorage.clear()
  vi.unstubAllGlobals()
})

describe('apiRequest token refresh', () => {
  it('rotates tokens then retries the original request once', async () => {
    localStorage.setItem('token', 'expired-access-token')
    localStorage.setItem('refresh_token', 'old-refresh-token')
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({ status: 401, ok: false })
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          access_token: 'new-access-token',
          refresh_token: 'new-refresh-token',
        }),
      })
      .mockResolvedValueOnce({ status: 200, ok: true })
    vi.stubGlobal('fetch', fetchMock)

    await expect(apiRequest('/assignments')).resolves.toMatchObject({ status: 200 })

    expect(fetchMock).toHaveBeenCalledTimes(3)
    expect(fetchMock.mock.calls[1][0]).toBe('/api/v1/auth/refresh')
    expect(JSON.parse(fetchMock.mock.calls[1][1].body)).toEqual({
      refresh_token: 'old-refresh-token',
    })
    expect(fetchMock.mock.calls[2][1].headers.get('Authorization')).toBe(
      'Bearer new-access-token',
    )
    expect(localStorage.getItem('token')).toBe('new-access-token')
    expect(localStorage.getItem('refresh_token')).toBe('new-refresh-token')
  })

  it('clears stored credentials when refreshing fails', async () => {
    localStorage.setItem('token', 'expired-access-token')
    localStorage.setItem('refresh_token', 'invalid-refresh-token')
    localStorage.setItem('user', JSON.stringify({ id: 'user-1' }))
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({ status: 401, ok: false })
      .mockResolvedValueOnce({ status: 401, ok: false })
    vi.stubGlobal('fetch', fetchMock)

    await expect(apiRequest('/assignments')).resolves.toMatchObject({ status: 401 })

    expect(localStorage.getItem('token')).toBeNull()
    expect(localStorage.getItem('refresh_token')).toBeNull()
    expect(localStorage.getItem('user')).toBeNull()
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })
})

describe('API errors', () => {
  it('surfaces the SRS error envelope message', async () => {
    const response = {
      status: 400,
      json: async () => ({
        error: { code: 'VALIDATION_ERROR', message: 'Batas memori tidak valid.' },
      }),
    }

    await expect(throwApiError(response, 'Permintaan gagal.')).rejects.toThrow(
      'Batas memori tidak valid.',
    )
  })

  it('explains rate limiting and honors Retry-After when supplied', async () => {
    const response = {
      status: 429,
      headers: new Headers({ 'Retry-After': '12' }),
      json: async () => ({}),
    }

    await expect(throwApiError(response, 'Permintaan gagal.')).rejects.toThrow(
      'Terlalu banyak permintaan. Coba lagi dalam 12 detik.',
    )
  })
})
