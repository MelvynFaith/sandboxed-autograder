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
      assignment_id: 'assignment-1',
      status: 'queued',
      submitted_at: '2026-10-05T10:00:00+07:00',
    }
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => submission,
    })
    vi.stubGlobal('fetch', fetchMock)

    await expect(submitSubmission('assignment-1', file)).resolves.toEqual(submission)

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
    ).rejects.toThrow('Gagal mengunggah kode (413). Silakan coba lagi.')
  })
})
