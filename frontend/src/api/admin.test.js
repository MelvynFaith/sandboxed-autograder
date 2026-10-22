import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  getAdminAuditLogs,
  getAdminUsers,
  getQueueStatus,
  getRateLimits,
  updateRateLimits,
} from './admin.js'

function stubFetch(payload, ok = true, status = 200) {
  const fetchMock = vi.fn().mockResolvedValue({ ok, status, json: async () => payload })
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

const rateLimits = {
  max_active_submissions: 1,
  min_submission_interval_s: 30,
  max_submissions_per_assignment: 30,
}

beforeEach(() => {
  localStorage.setItem('token', 'admin-token')
  vi.stubEnv('VITE_USE_MOCK', 'false')
})

afterEach(() => {
  localStorage.clear()
  vi.unstubAllEnvs()
  vi.unstubAllGlobals()
})

describe('admin list pagination', () => {
  it.each([
    ['users', getAdminUsers, '/api/v1/admin/users?page=2&page_size=10'],
    ['audit logs', getAdminAuditLogs, '/api/v1/admin/audit-logs?page=2&page_size=10'],
  ])('requests a selected page of %s', async (_name, getList, expectedUrl) => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ items: [], page: 2, page_size: 10, has_next: false }),
    })
    vi.stubGlobal('fetch', fetchMock)

    await expect(getList(2, 10)).resolves.toMatchObject({
      items: [],
      page: 2,
      pageSize: 10,
      hasNextPage: false,
    })
    expect(fetchMock).toHaveBeenCalledWith(
      expectedUrl,
      expect.objectContaining({ headers: expect.any(Headers) }),
    )
  })
})

describe('admin queue status', () => {
  it('reads the queue and worker counts', async () => {
    const fetchMock = stubFetch({
      queue_depth: 4,
      workers: { alive: 1, configured: 2 },
      submissions: { queued: 4, running: 1 },
    })

    await expect(getQueueStatus()).resolves.toEqual({
      queueDepth: 4,
      workersAlive: 1,
      workersConfigured: 2,
      queued: 4,
      running: 1,
    })
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/admin/queue')
  })

  it('rejects a response without the worker counts', async () => {
    stubFetch({ queue_depth: 0, submissions: { queued: 0, running: 0 } })

    await expect(getQueueStatus()).rejects.toThrow(
      'Respons status antrian tidak sesuai format yang diharapkan.',
    )
  })

  it('shows the API error message', async () => {
    stubFetch({ error: { code: 'not_found', message: 'Endpoint tidak ditemukan.' } }, false, 404)

    await expect(getQueueStatus()).rejects.toThrow('Endpoint tidak ditemukan.')
  })
})

describe('admin rate limits', () => {
  it('reads the three thresholds and drops other fields', async () => {
    const fetchMock = stubFetch({ ...rateLimits, updated_at: '2026-10-19T00:00:00Z' })

    await expect(getRateLimits()).resolves.toEqual(rateLimits)
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/admin/rate-limits')
  })

  it.each([
    ['a negative value', { ...rateLimits, max_active_submissions: -1 }],
    ['an interval above one day', { ...rateLimits, min_submission_interval_s: 86401 }],
    ['a missing field', { max_active_submissions: 1, min_submission_interval_s: 30 }],
  ])('rejects a response with %s', async (_name, payload) => {
    stubFetch(payload)

    await expect(getRateLimits()).rejects.toThrow(
      'Respons batas laju tidak sesuai format yang diharapkan.',
    )
  })

  it('saves with PUT and a body of exactly the three thresholds', async () => {
    const saved = { ...rateLimits, max_active_submissions: 0, min_submission_interval_s: 60 }
    const fetchMock = stubFetch(saved)

    await expect(updateRateLimits({ ...saved, updated_at: 'ignored' })).resolves.toEqual(saved)
    const [url, options] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/v1/admin/rate-limits')
    expect(options.method).toBe('PUT')
    expect(JSON.parse(options.body)).toEqual(saved)
  })

  it('validates the saved values in mock mode like the API', async () => {
    vi.stubEnv('VITE_USE_MOCK', 'true')

    await expect(
      updateRateLimits({ ...rateLimits, max_submissions_per_assignment: 10001 }),
    ).rejects.toThrow('Nilai batas laju di luar rentang yang diizinkan.')
    await expect(getRateLimits()).resolves.toEqual(rateLimits)
  })
})
