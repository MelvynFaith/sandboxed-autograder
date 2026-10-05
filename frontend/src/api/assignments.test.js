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
      { id: 'a1', judul: 'Algoritma', deskripsi: 'Deskripsi', deadline: '2026-10-12' },
    ]
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => assignments,
    })
    vi.stubGlobal('fetch', fetchMock)

    await expect(getAssignments()).resolves.toEqual(assignments)
    expect(fetchMock).toHaveBeenCalledWith('/api/v1/assignments', expect.any(Object))
  })

  it('surfaces a failed assignment request', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 503 }))

    await expect(getAssignments()).rejects.toThrow('Gagal memuat assignment (503).')
  })

  it('rejects assignments with an invalid deadline', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: async () => [
        { id: 'a1', judul: 'Algoritma', deskripsi: 'Deskripsi', deadline: 'not-a-date' },
      ],
    }))

    await expect(getAssignments()).rejects.toThrow(
      'Respons daftar assignment tidak sesuai format yang diharapkan.',
    )
  })
})
