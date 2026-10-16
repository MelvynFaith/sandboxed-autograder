import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { submitSubmission } from './submissions.js'

beforeEach(() => {
  localStorage.clear()
  vi.stubEnv('VITE_USE_MOCK', 'false')
})

afterEach(() => {
  localStorage.clear()
  vi.unstubAllEnvs()
  vi.unstubAllGlobals()
})

describe('submitSubmission', () => {
  it('posts a multipart submission with the expected fields', async () => {
    const file = new File(['print("ok")'], 'solution.py', { type: 'text/x-python' })
    const submission = {
      id: 'submission-1',
      status: 'QUEUED',
      is_late: false,
      submitted_at: '2026-10-05T10:00:00+07:00',
    }
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => submission,
    })
    vi.stubGlobal('fetch', fetchMock)

    await expect(submitSubmission('assignment-1', file)).resolves.toEqual({
      id: 'submission-1',
      status: 'QUEUED',
      isLate: false,
      submittedAt: '2026-10-05T10:00:00+07:00',
    })

    const [url, options] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/v1/submissions')
    expect(options.method).toBe('POST')
    expect(options.body).toBeInstanceOf(FormData)
    expect(options.body.get('assignment_id')).toBe('assignment-1')
    expect(options.body.get('file')).toBe(file)
    expect(options.headers.has('Content-Type')).toBe(false)
  })

  it('surfaces a failed submission request', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 413 }))

    await expect(
      submitSubmission('assignment-1', new File(['print(1)'], 'solution.py')),
    ).rejects.toThrow('Ukuran berkas melebihi batas maksimum 256 KB.')
  })

  it.each([
    [409, 'Submission ditolak karena batas waktu pengumpulan telah lewat.'],
    [413, 'Ukuran berkas melebihi batas maksimum 256 KB.'],
    [429, 'Terlalu sering mengunggah. Silakan tunggu sebelum mencoba lagi.'],
  ])('shows a specific message for upload HTTP %s', async (status, message) => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status }))

    await expect(
      submitSubmission('assignment-1', new File(['print(1)'], 'solution.py')),
    ).rejects.toThrow(message)
  })

  it('prefers an SRS error-envelope message over the upload status fallback', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      status: 409,
      json: async () => ({
        error: { code: 'SUBMISSION_TOO_LATE', message: 'Pengumpulan sudah ditutup.' },
      }),
    }))

    await expect(
      submitSubmission('assignment-1', new File(['print(1)'], 'solution.py')),
    ).rejects.toThrow('Pengumpulan sudah ditutup.')
  })

  it('includes the retry delay for rate-limited uploads', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      status: 429,
      headers: new Headers({ 'Retry-After': '30' }),
    }))

    await expect(
      submitSubmission('assignment-1', new File(['print(1)'], 'solution.py')),
    ).rejects.toThrow('Terlalu sering mengunggah. Coba lagi dalam 30 detik.')
  })
})
