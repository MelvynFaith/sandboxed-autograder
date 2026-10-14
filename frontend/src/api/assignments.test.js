import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { getAssignments } from './assignments.js'

beforeEach(() => {
  localStorage.clear()
  vi.stubEnv('VITE_USE_MOCK', 'false')
})

afterEach(() => {
  localStorage.clear()
  vi.unstubAllEnvs()
  vi.unstubAllGlobals()
})

describe('getAssignments', () => {
  it('gets and validates the assignment list', async () => {
    const assignments = [
      {
        id: 'a1',
        course_id: 'c1',
        title: 'Algoritma',
        description: 'Deskripsi',
        deadline_at: '2026-10-12T16:59:00Z',
        late_until_at: '2026-10-13T16:59:00Z',
      },
    ]
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => assignments,
    })
    vi.stubGlobal('fetch', fetchMock)

    await expect(getAssignments({ courseId: 'c1' })).resolves.toEqual([
      {
        id: 'a1',
        courseId: 'c1',
        title: 'Algoritma',
        description: 'Deskripsi',
        deadlineAt: '2026-10-12T16:59:00Z',
        lateUntilAt: '2026-10-13T16:59:00Z',
      },
    ])
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/courses/c1/assignments?page=1&page_size=50',
      expect.any(Object),
    )
  })

  it('surfaces a failed assignment request', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 503 }))

    await expect(getAssignments()).rejects.toThrow('Gagal memuat assignment (503).')
  })

  it('rejects assignments with an invalid deadline', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: async () => [
        {
          id: 'a1',
          course_id: 'c1',
          title: 'Algoritma',
          description: 'Deskripsi',
          deadline_at: 'not-a-date',
          late_until_at: '2026-10-13T16:59:00Z',
        },
      ],
    }))

    await expect(getAssignments()).rejects.toThrow(
      'Respons daftar assignment tidak sesuai format yang diharapkan.',
    )
  })
})
