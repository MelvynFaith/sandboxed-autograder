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
      total_score: 75,
      max_score: 100,
      results: [
        {
          testcase_id: 'testcase-1',
          external_id: 'tc_01',
          status: 'TIMEOUT',
          score: 0,
          exec_time_ms: 5000,
          memory_kb: 4096,
          error_message: 'Batas waktu terlampaui',
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
      maxScore: 100,
      results: [
        {
          testCaseId: 'testcase-1',
          externalId: 'tc_01',
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

  it('maps a hidden student result that has no testcase_id', async () => {
    vi.stubEnv('VITE_USE_MOCK', 'true')

    const report = await getSubmissionReport('submission-1')
    const hidden = report.results.find((result) => result.hidden)

    expect(report).toMatchObject({ totalScore: 85, maxScore: 100 })
    expect(hidden).toEqual({
      testCaseId: null,
      externalId: null,
      verdict: 'SECURITY_VIOLATION',
      score: 0,
      executionTimeMs: 1,
      memoryKb: 1024,
      hidden: true,
      errorMessage: null,
      actualOutput: null,
    })
  })

  it('drops hidden output and error messages even if the server sent them', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        submission_id: 'submission-1',
        total_score: 0,
        max_score: 10,
        results: [
          {
            testcase_id: 9,
            external_id: 'tc_secret',
            status: 'FAILED',
            score: 0,
            exec_time_ms: 3,
            memory_kb: 100,
            is_hidden: true,
            error_message: 'Pesan tersembunyi',
            actual_output: 'OUTPUT-TERSEMBUNYI',
          },
        ],
      }),
    }))

    const report = await getSubmissionReport('submission-1')

    expect(report.results[0]).toMatchObject({
      hidden: true,
      testCaseId: null,
      externalId: null,
      errorMessage: null,
      actualOutput: null,
    })
    expect(JSON.stringify(report)).not.toContain('OUTPUT-TERSEMBUNYI')
    expect(JSON.stringify(report)).not.toContain('Pesan tersembunyi')
    expect(JSON.stringify(report)).not.toContain('tc_secret')
  })

  it('accepts a report with no scores before the submission is graded', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ submission_id: 5, results: [] }),
    }))

    await expect(getSubmissionReport('5')).resolves.toEqual({
      submissionId: '5',
      totalScore: null,
      maxScore: null,
      results: [],
    })
  })

  it('still requires testcase_id on a visible result', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        submission_id: 'submission-1',
        total_score: 0,
        results: [
          {
            status: 'FAILED',
            score: 0,
            exec_time_ms: 1,
            memory_kb: 1,
            is_hidden: false,
          },
        ],
      }),
    }))

    await expect(getSubmissionReport('submission-1')).rejects.toThrow(
      'Data hasil test case tidak sesuai format yang diharapkan.',
    )
  })

  it('shows returned testcase details when visibility metadata is not in the SRS response', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        submission_id: 'submission-1',
        total_score: 10,
        results: [
          {
            testcase_id: 'testcase-hidden',
            status: 'PASSED',
            score: 10,
            exec_time_ms: 12,
            memory_kb: 2048,
            actual_output: 'secret output',
          },
        ],
      }),
    }))

    await expect(getSubmissionReport('submission-1')).resolves.toMatchObject({
      results: [
        {
          testCaseId: 'testcase-hidden',
          externalId: null,
          hidden: false,
          errorMessage: null,
          actualOutput: 'secret output',
        },
      ],
    })
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
          total_score: 0,
          results: [
            {
              testcase_id: 'testcase-1',
              status: 'UNKNOWN',
              score: 0,
              exec_time_ms: 1,
              memory_kb: 1,
              error_message: null,
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
