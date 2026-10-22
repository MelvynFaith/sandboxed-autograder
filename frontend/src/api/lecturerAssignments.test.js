import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  createLecturerAssignment,
  getResourceLimit,
  updateLecturerAssignment,
  updateResourceLimit,
} from './lecturerAssignments.js'

const assignmentFields = {
  title: 'Latihan Algoritma',
  description: 'Latihan dasar',
  deadline_at: '2026-10-12T16:59:00.000Z',
  late_until_at: '2026-10-13T16:59:00.000Z',
}

const assignmentResponse = { id: 7, course_id: 3, ...assignmentFields }

beforeEach(() => {
  localStorage.setItem('token', 'lecturer-token')
  vi.stubEnv('VITE_USE_MOCK', 'false')
})

afterEach(() => {
  localStorage.clear()
  vi.unstubAllEnvs()
  vi.unstubAllGlobals()
})

describe('assignment API', () => {
  it('creates an assignment with its resource limit in the same body', async () => {
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
      json: async () => assignmentResponse,
    })
    vi.stubGlobal('fetch', fetchMock)

    await expect(createLecturerAssignment('3', assignmentFields, resourceLimit)).resolves.toEqual({
      id: '7',
      courseId: '3',
      title: 'Latihan Algoritma',
      description: 'Latihan dasar',
      deadlineAt: '2026-10-12T16:59:00.000Z',
      lateUntilAt: '2026-10-13T16:59:00.000Z',
    })
    expect(fetchMock).toHaveBeenCalledTimes(1)
    const [url, options] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/v1/courses/3/assignments')
    expect(options.method).toBe('POST')
    expect(JSON.parse(options.body)).toEqual({ ...assignmentFields, resource_limit: resourceLimit })
  })

  it('updates an assignment with only the fields the API accepts', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => assignmentResponse,
    })
    vi.stubGlobal('fetch', fetchMock)

    await expect(updateLecturerAssignment('7', assignmentFields)).resolves.toMatchObject({
      id: '7',
      courseId: '3',
    })
    const [url, options] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/v1/assignments/7')
    expect(options.method).toBe('PUT')
    expect(JSON.parse(options.body)).toEqual(assignmentFields)
  })
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
