import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cancelSubmission, getSubmissionHistory } from './submissionHistory.js'

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
        max_score: 100,
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
        maxScore: 100,
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

  it('accepts a submission that is not graded yet, with no score fields', async () => {
    const items = ['QUEUED', 'RUNNING', 'CANCELLED'].map((status, index) => ({
      id: index + 1,
      assignment_id: 7,
      assignment_title: 'Struktur Data',
      status,
      is_late: false,
      submitted_at: '2026-10-05T02:15:00Z',
    }))
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({ items, page: 1, page_size: 50, has_next: false }),
      }),
    )

    const result = await getSubmissionHistory()

    expect(result.items).toHaveLength(3)
    expect(result.items[0]).toMatchObject({
      id: '1',
      assignmentId: '7',
      assignmentTitle: 'Struktur Data',
      status: 'QUEUED',
      totalScore: null,
      maxScore: null,
    })
  })

  it('reads the mock history, which includes ungraded submissions', async () => {
    vi.stubEnv('VITE_USE_MOCK', 'true')

    const result = await getSubmissionHistory()

    expect(result.items.map((entry) => entry.status)).toEqual(['QUEUED', 'RUNNING', 'COMPLETED'])
    expect(result.items[0].totalScore).toBeNull()
    expect(result.items[2]).toMatchObject({ totalScore: 85, maxScore: 100 })
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

describe('cancelSubmission', () => {
  const cancelled = {
    id: 12,
    assignment_id: 3,
    assignment_title: 'Dasar Algoritma',
    status: 'CANCELLED',
    is_late: false,
    submitted_at: '2026-10-04T10:30:00Z',
  }

  it('sends DELETE with the auth header and returns the cancelled submission', async () => {
    const fetchMock = vi.fn().mockResolvedValue({ ok: true, json: async () => cancelled })
    vi.stubGlobal('fetch', fetchMock)

    await expect(cancelSubmission('12')).resolves.toMatchObject({
      id: '12',
      status: 'CANCELLED',
      totalScore: null,
    })
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/submissions/12',
      expect.objectContaining({ method: 'DELETE', headers: expect.any(Headers) }),
    )
    expect(fetchMock.mock.calls[0][1].headers.get('Authorization')).toBe('Bearer history-token')
  })

  it('shows the API message when the submission already started (409)', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: false,
        status: 409,
        json: async () => ({
          error: {
            code: 'not_cancellable',
            message: 'Submission yang sedang dinilai tidak dapat dibatalkan.',
          },
        }),
      }),
    )

    await expect(cancelSubmission('12')).rejects.toThrow(
      'Submission yang sedang dinilai tidak dapat dibatalkan.',
    )
  })

  it('falls back to a fixed 409 message when the body has no error envelope', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: false,
        status: 409,
        json: async () => {
          throw new SyntaxError('no body')
        },
      }),
    )

    await expect(cancelSubmission('12')).rejects.toThrow(
      'Submission tidak dapat dibatalkan karena sudah mulai dinilai.',
    )
  })

  it('rejects a response for another submission or with a bad shape', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: async () => cancelled }))
    await expect(cancelSubmission('99')).rejects.toThrow(
      'Respons pembatalan submission tidak sesuai format yang diharapkan.',
    )

    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: async () => ({ id: 12 }) }))
    await expect(cancelSubmission('12')).rejects.toThrow(
      'Respons pembatalan submission tidak sesuai format yang diharapkan.',
    )
  })

  it('cancels a queued mock submission and refuses a running one', async () => {
    vi.stubEnv('VITE_USE_MOCK', 'true')

    await expect(cancelSubmission('submission-3')).resolves.toMatchObject({
      id: 'submission-3',
      status: 'CANCELLED',
    })
    const history = await getSubmissionHistory()
    expect(history.items.find((entry) => entry.id === 'submission-3').status).toBe('CANCELLED')

    await expect(cancelSubmission('submission-2')).rejects.toThrow(
      'Submission yang sedang dinilai tidak dapat dibatalkan.',
    )
    await expect(cancelSubmission('submission-1')).rejects.toThrow(
      'Hanya submission yang masih antre yang dapat dibatalkan.',
    )
    await expect(cancelSubmission('missing')).rejects.toThrow('Submission tidak ditemukan.')
  })
})
