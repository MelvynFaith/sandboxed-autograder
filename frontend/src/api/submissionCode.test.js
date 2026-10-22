import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { getSubmissionCode } from './submissionCode.js'

beforeEach(() => {
  localStorage.setItem('token', 'code-token')
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

describe('getSubmissionCode', () => {
  it('fetches the code endpoint with the auth header and maps the payload', async () => {
    const fetchMock = stubFetch({
      ok: true,
      json: async () => ({ submission_id: 9, original_filename: 'a.py', code: 'print(1)\n' }),
    })

    const result = await getSubmissionCode('9')

    expect(result).toEqual({ submissionId: '9', originalFilename: 'a.py', code: 'print(1)\n' })
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/submissions/9/code')
    expect(fetchMock.mock.calls[0][1].headers.get('Authorization')).toBe('Bearer code-token')
  })

  it('shows the API message when the code was deleted (410)', async () => {
    stubFetch({
      ok: false,
      status: 410,
      json: async () => ({
        error: { code: 'code_deleted', message: 'Berkas kode sudah dihapus karena masa retensi.' },
      }),
    })

    await expect(getSubmissionCode('9')).rejects.toThrow(
      'Berkas kode sudah dihapus karena masa retensi.',
    )
  })

  it('falls back to fixed messages when the error body is not the API envelope', async () => {
    stubFetch({ ok: false, status: 410, json: async () => ({}) })
    await expect(getSubmissionCode('9')).rejects.toThrow('masa retensi')

    stubFetch({ ok: false, status: 404, json: async () => ({}) })
    await expect(getSubmissionCode('9')).rejects.toThrow('Submission tidak ditemukan.')
  })

  it('rejects a response for another submission or with a bad shape', async () => {
    stubFetch({
      ok: true,
      json: async () => ({ submission_id: 1, original_filename: 'a.py', code: '' }),
    })
    await expect(getSubmissionCode('9')).rejects.toThrow('tidak sesuai format')

    stubFetch({ ok: true, json: async () => ({ submission_id: 9, original_filename: 'a.py' }) })
    await expect(getSubmissionCode('9')).rejects.toThrow('tidak sesuai format')
  })

  it('serves mock code and the retention error in mock mode', async () => {
    vi.stubEnv('VITE_USE_MOCK', 'true')

    expect((await getSubmissionCode('s-1')).code).toContain('def solve')
    await expect(getSubmissionCode('s-deleted')).rejects.toThrow('masa retensi')
  })
})
