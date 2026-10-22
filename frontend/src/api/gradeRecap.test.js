import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  downloadRecapCsv,
  getAssignmentRecap,
  regradeAssignment,
  regradeSubmission,
} from './gradeRecap.js'

beforeEach(() => {
  localStorage.setItem('token', 'recap-token')
  vi.stubEnv('VITE_USE_MOCK', 'false')
})

afterEach(() => {
  localStorage.clear()
  vi.unstubAllEnvs()
  vi.unstubAllGlobals()
})

function stubFetch(response) {
  const fetchMock = vi.fn().mockResolvedValue(response)
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

const recapPayload = {
  assignment_id: 4,
  assignment_title: 'Dasar Algoritma',
  items: [
    {
      user_id: 1,
      name: 'Ayu',
      email: 'ayu@example.test',
      submission_count: 2,
      latest_submission: {
        id: 11,
        status: 'COMPLETED',
        total_score: 80,
        max_score: 100,
        is_late: true,
        stale_grade: true,
        submitted_at: '2026-10-04T03:30:00Z',
      },
    },
    {
      user_id: 2,
      name: 'Budi',
      email: 'budi@example.test',
      submission_count: 0,
      latest_submission: null,
    },
  ],
  page: 2,
  page_size: 10,
  has_next: true,
}

describe('getAssignmentRecap', () => {
  it('requests the page and maps rows, including a student with no submission', async () => {
    const fetchMock = stubFetch({ ok: true, json: async () => recapPayload })

    const result = await getAssignmentRecap('4', { page: 2, pageSize: 10 })

    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/assignments/4/results?page=2&page_size=10')
    expect(result).toMatchObject({
      assignmentId: '4',
      assignmentTitle: 'Dasar Algoritma',
      page: 2,
      pageSize: 10,
      hasNextPage: true,
    })
    expect(result.items[0]).toEqual({
      userId: '1',
      name: 'Ayu',
      email: 'ayu@example.test',
      submissionCount: 2,
      latestSubmission: {
        id: '11',
        status: 'COMPLETED',
        totalScore: 80,
        maxScore: 100,
        isLate: true,
        staleGrade: true,
        submittedAt: '2026-10-04T03:30:00Z',
      },
    })
    expect(result.items[1].latestSubmission).toBeNull()
  })

  it('accepts a latest submission that has no scores yet', async () => {
    const row = {
      ...recapPayload.items[0],
      latest_submission: {
        id: 12,
        status: 'QUEUED',
        is_late: false,
        stale_grade: false,
        submitted_at: '2026-10-05T02:00:00Z',
      },
    }
    stubFetch({ ok: true, json: async () => ({ ...recapPayload, items: [row] }) })

    const result = await getAssignmentRecap('4', { page: 2, pageSize: 10 })

    expect(result.items[0].latestSubmission).toMatchObject({ totalScore: null, maxScore: null })
  })

  it('surfaces API errors, bad shapes and invalid paging', async () => {
    stubFetch({
      ok: false,
      status: 403,
      json: async () => ({ error: { code: 'forbidden', message: 'Akses ditolak.' } }),
    })
    await expect(getAssignmentRecap('4')).rejects.toThrow('Akses ditolak.')

    stubFetch({ ok: true, json: async () => ({ ...recapPayload, assignment_id: 5 }) })
    await expect(getAssignmentRecap('4')).rejects.toThrow('tidak sesuai format')

    stubFetch({ ok: true, json: async () => ({ ...recapPayload, items: [{ user_id: 1 }] }) })
    await expect(getAssignmentRecap('4')).rejects.toThrow('tidak sesuai format')

    await expect(getAssignmentRecap('4', { page: 0 })).rejects.toThrow('bilangan bulat positif')
  })
})

describe('downloadRecapCsv', () => {
  let clicked

  beforeEach(() => {
    clicked = []
    URL.createObjectURL = vi.fn(() => 'blob:csv')
    URL.revokeObjectURL = vi.fn()
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function click() {
      clicked.push({ href: this.href, download: this.download })
    })
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('fetches the export with the auth header and saves the blob under the server file name', async () => {
    const blob = new Blob(['name,email\r\n'], { type: 'text/csv' })
    const fetchMock = stubFetch({
      ok: true,
      headers: new Headers({
        'Content-Disposition': 'attachment; filename="rekap-assignment-4.csv"',
      }),
      blob: async () => blob,
    })

    const filename = await downloadRecapCsv('4')

    expect(filename).toBe('rekap-assignment-4.csv')
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/assignments/4/results/export')
    expect(fetchMock.mock.calls[0][1].headers.get('Authorization')).toBe('Bearer recap-token')
    expect(URL.createObjectURL).toHaveBeenCalledWith(blob)
    expect(clicked).toEqual([{ href: 'blob:csv', download: 'rekap-assignment-4.csv' }])
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:csv')
  })

  it('ignores a file name with a path and falls back to the default', async () => {
    stubFetch({
      ok: true,
      headers: new Headers({ 'Content-Disposition': 'attachment; filename="../evil.csv"' }),
      blob: async () => new Blob(['x']),
    })

    expect(await downloadRecapCsv('4')).toBe('rekap-assignment-4.csv')
  })

  it('does not save anything when the export fails', async () => {
    stubFetch({
      ok: false,
      status: 404,
      json: async () => ({ error: { code: 'not_found', message: 'Assignment tidak ditemukan.' } }),
    })

    await expect(downloadRecapCsv('4')).rejects.toThrow('Assignment tidak ditemukan.')
    expect(clicked).toEqual([])
  })
})

describe('regrade', () => {
  it('POSTs a single regrade and returns the queued submission', async () => {
    const fetchMock = stubFetch({
      ok: true,
      json: async () => ({ id: 11, status: 'QUEUED' }),
    })

    expect(await regradeSubmission('11')).toEqual({ id: '11', status: 'QUEUED' })
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/submissions/11/regrade')
    expect(fetchMock.mock.calls[0][1].method).toBe('POST')
  })

  it('shows the API message for a state conflict and a deleted code file', async () => {
    stubFetch({
      ok: false,
      status: 409,
      json: async () => ({ error: { code: 'invalid_status', message: 'Status tidak valid.' } }),
    })
    await expect(regradeSubmission('11')).rejects.toThrow('Status tidak valid.')

    stubFetch({ ok: false, status: 410, json: async () => ({}) })
    await expect(regradeSubmission('11')).rejects.toThrow('masa retensi')
  })

  it('rejects a response for another submission', async () => {
    stubFetch({ ok: true, json: async () => ({ id: 99, status: 'QUEUED' }) })
    await expect(regradeSubmission('11')).rejects.toThrow('tidak sesuai format')
  })

  it('POSTs a whole-assignment regrade and returns the counts', async () => {
    const fetchMock = stubFetch({
      ok: true,
      json: async () => ({ assignment_id: 4, queued: 3, skipped: 1 }),
    })

    expect(await regradeAssignment('4')).toEqual({ assignmentId: '4', queued: 3, skipped: 1 })
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/assignments/4/regrade')
    expect(fetchMock.mock.calls[0][1].method).toBe('POST')

    stubFetch({ ok: true, json: async () => ({ assignment_id: 4, queued: 'x' }) })
    await expect(regradeAssignment('4')).rejects.toThrow('tidak sesuai format')
  })
})
