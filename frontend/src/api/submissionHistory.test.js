import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { getSubmissionHistory } from './submissionHistory.js'

beforeEach(() => {
  localStorage.setItem('token', 'history-token')
  vi.stubEnv('VITE_USE_MOCK', 'false')
})

afterEach(() => {
  localStorage.clear()
  vi.unstubAllEnvs()
  vi.unstubAllGlobals()
})

describe('getSubmissionHistory', () => {
  it('fetches the history endpoint and maps entries to the UI model', async () => {
    const payload = [
      {
        id: 'submission-1',
        assignment_id: 'assignment-1',
        assignment_title: 'Dasar Algoritma',
        status: 'COMPLETED',
        total_score: 85,
        is_late: false,
        submitted_at: '2026-10-04T10:30:00+07:00',
      },
    ]
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => payload,
    })
    vi.stubGlobal('fetch', fetchMock)

    await expect(getSubmissionHistory()).resolves.toEqual({
      items: [{
        id: 'submission-1',
        assignmentId: 'assignment-1',
        assignmentTitle: 'Dasar Algoritma',
        status: 'COMPLETED',
        totalScore: 85,
        isLate: false,
        submittedAt: '2026-10-04T10:30:00+07:00',
      }],
      page: 1,
      pageSize: 50,
      hasNextPage: false,
    })
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/submissions?page=1&page_size=50',
      expect.objectContaining({ headers: expect.any(Headers) }),
    )
    expect(fetchMock.mock.calls[0][1].headers.get('Authorization')).toBe(
      'Bearer history-token',
    )
  })

  it('accepts an empty history', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: async () => [] }))

    await expect(getSubmissionHistory()).resolves.toMatchObject({
      items: [],
      hasNextPage: false,
    })
  })

  it('requests the selected page and reads the server pagination metadata', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        items: [],
        page: 2,
        page_size: 10,
        has_next: true,
      }),
    })
    vi.stubGlobal('fetch', fetchMock)

    await expect(getSubmissionHistory({ page: 2, pageSize: 10 })).resolves.toEqual({
      items: [],
      page: 2,
      pageSize: 10,
      hasNextPage: true,
    })
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/submissions?page=2&page_size=10',
      expect.objectContaining({ headers: expect.any(Headers) }),
    )
  })

  it('surfaces failed requests and invalid response shapes', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 500 }))

    await expect(getSubmissionHistory()).rejects.toThrow(
      'Gagal memuat riwayat submission (500).',
    )

    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({ ok: true, json: async () => [{ id: 'incomplete' }] }),
    )
    await expect(getSubmissionHistory()).rejects.toThrow(
      'Respons riwayat submission tidak sesuai format yang diharapkan.',
    )
  })
})
