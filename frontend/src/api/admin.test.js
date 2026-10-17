import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { getAdminAuditLogs, getAdminUsers } from './admin.js'

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
