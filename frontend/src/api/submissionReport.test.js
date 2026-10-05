import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { getSubmissionReport } from './submissionReport.js'

beforeEach(() => {
  localStorage.setItem('token', 'report-token')
  vi.stubEnv('VITE_USE_MOCK', 'false')
})

afterEach(() => {
  localStorage.clear()
  vi.unstubAllEnvs()
  vi.unstubAllGlobals()
})

describe('getSubmissionReport', () => {
  it('fetches the report endpoint and maps a visible testcase result', async () => {
    const payload = {
      submission_id: 'submission-1',
      skor_total: 75,
      results: [
        {
          testcase_id: 'testcase-1',
          status: 'TIMEOUT',
          skor: 0,
          waktu_eksekusi_ms: 5000,
          memori_kb: 4096,
          pesan_error: 'Batas waktu terlampaui',
          is_hidden: false,
          actual_output: '',
        },
      ],
    }
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => payload,
    })
    vi.stubGlobal('fetch', fetchMock)

    await expect(getSubmissionReport('submission-1')).resolves.toEqual({
      submissionId: 'submission-1',
      totalScore: 75,
      results: [
        {
          testCaseId: 'testcase-1',
          verdict: 'TIMEOUT',
          score: 0,
          executionTimeMs: 5000,
          memoryKb: 4096,
          hidden: false,
          errorMessage: 'Batas waktu terlampaui',
          actualOutput: '',
        },
      ],
    })
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/submissions/submission-1/report',
      expect.objectContaining({ headers: expect.any(Headers) }),
    )
    expect(fetchMock.mock.calls[0][1].headers.get('Authorization')).toBe(
      'Bearer report-token',
    )
  })

  it('does not expose hidden testcase output or error messages in the UI model', async () => {
    vi.stubEnv('VITE_USE_MOCK', 'true')

    const report = await getSubmissionReport('submission-1')
    const hidden = report.results.find((result) => result.hidden)

    expect(hidden).toMatchObject({
      verdict: 'SECURITY_VIOLATION',
      score: 0,
      executionTimeMs: 1,
      memoryKb: 1024,
      errorMessage: null,
      actualOutput: null,
    })
    expect(JSON.stringify(hidden)).not.toContain('OUTPUT-TERSEMBUNYI-MOCK')
    expect(JSON.stringify(hidden)).not.toContain('Pesan tersembunyi')
  })

  it('rejects unknown verdicts and surfaces failed requests', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 404 }))

    await expect(getSubmissionReport('submission-1')).rejects.toThrow(
      'Gagal memuat laporan submission (404).',
    )

    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({
          submission_id: 'submission-1',
          skor_total: 0,
          results: [
            {
              testcase_id: 'testcase-1',
              status: 'UNKNOWN',
              skor: 0,
              waktu_eksekusi_ms: 1,
              memori_kb: 1,
              pesan_error: null,
              is_hidden: false,
              actual_output: null,
            },
          ],
        }),
      }),
    )
    await expect(getSubmissionReport('submission-1')).rejects.toThrow(
      'Data hasil test case tidak sesuai format yang diharapkan.',
    )
  })
})
