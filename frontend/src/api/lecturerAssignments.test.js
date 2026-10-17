import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { getResourceLimit, updateResourceLimit } from './lecturerAssignments.js'

beforeEach(() => {
  localStorage.setItem('token', 'lecturer-token')
  vi.stubEnv('VITE_USE_MOCK', 'false')
})

afterEach(() => {
  localStorage.clear()
  vi.unstubAllEnvs()
  vi.unstubAllGlobals()
})

describe('resource-limit API', () => {
  it('updates resource limits with PUT on the SRS endpoint', async () => {
    const resourceLimit = {
      cpu_time_ms: 1000,
      memory_mb: 64,
      max_processes: 4,
      timeout_s: 5,
      max_output_bytes: 1048576,
      tmpfs_size_mb: 32,
    }
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => resourceLimit,
    })
    vi.stubGlobal('fetch', fetchMock)

    await expect(updateResourceLimit('assignment-1', resourceLimit)).resolves.toEqual(resourceLimit)

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/assignments/assignment-1/resource-limit',
      expect.objectContaining({
        method: 'PUT',
        body: JSON.stringify(resourceLimit),
      }),
    )
  })

  it('reads resource limits from the same SRS endpoint', async () => {
    const resourceLimit = {
      cpu_time_ms: 1000,
      memory_mb: 64,
      max_processes: 4,
      timeout_s: 5,
      max_output_bytes: 1048576,
      tmpfs_size_mb: 32,
    }
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => resourceLimit,
    })
    vi.stubGlobal('fetch', fetchMock)

    await expect(getResourceLimit('assignment-1')).resolves.toEqual(resourceLimit)
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/assignments/assignment-1/resource-limit',
      expect.objectContaining({ headers: expect.any(Headers) }),
    )
  })
})
