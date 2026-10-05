import { apiRequest } from './client.js'

const verdicts = new Set([
  'PASSED',
  'FAILED',
  'TIMEOUT',
  'MEMORY_LIMIT_EXCEEDED',
  'RUNTIME_ERROR',
  'SYNTAX_ERROR',
  'SECURITY_VIOLATION',
])

const mockSubmissionReport = {
  submission_id: 'submission-1',
  skor_total: 85,
  results: [
    {
      testcase_id: 'testcase-1',
      status: 'PASSED',
      skor: 25,
      waktu_eksekusi_ms: 12,
      memori_kb: 2048,
      pesan_error: null,
      is_hidden: false,
      actual_output: '42',
    },
    {
      testcase_id: 'testcase-2',
      status: 'RUNTIME_ERROR',
      skor: 0,
      waktu_eksekusi_ms: 18,
      memori_kb: 4096,
      pesan_error: 'TypeError: unsupported operand type',
      is_hidden: false,
      actual_output: '',
    },
    {
      testcase_id: 'testcase-3',
      status: 'SECURITY_VIOLATION',
      skor: 0,
      waktu_eksekusi_ms: 1,
      memori_kb: 1024,
      pesan_error: 'Pesan tersembunyi',
      is_hidden: true,
      actual_output: 'OUTPUT-TERSEMBUNYI-MOCK',
    },
    {
      testcase_id: 'testcase-4',
      status: 'FAILED',
      skor: 0,
      waktu_eksekusi_ms: 14,
      memori_kb: 2048,
      pesan_error: null,
      is_hidden: false,
      actual_output: '0',
    },
    {
      testcase_id: 'testcase-5',
      status: 'TIMEOUT',
      skor: 0,
      waktu_eksekusi_ms: 5000,
      memori_kb: 8192,
      pesan_error: 'Batas waktu terlampaui',
      is_hidden: false,
      actual_output: null,
    },
    {
      testcase_id: 'testcase-6',
      status: 'MEMORY_LIMIT_EXCEEDED',
      skor: 0,
      waktu_eksekusi_ms: 28,
      memori_kb: 65536,
      pesan_error: 'Batas memori terlampaui',
      is_hidden: false,
      actual_output: null,
    },
    {
      testcase_id: 'testcase-7',
      status: 'SYNTAX_ERROR',
      skor: 0,
      waktu_eksekusi_ms: 0,
      memori_kb: 0,
      pesan_error: 'SyntaxError',
      is_hidden: false,
      actual_output: null,
    },
  ],
}

function isIdentifier(value) {
  return typeof value === 'string' || typeof value === 'number'
}

function readResult(result) {
  if (
    !result ||
    typeof result !== 'object' ||
    !isIdentifier(result.testcase_id) ||
    !verdicts.has(result.status) ||
    !Number.isFinite(result.skor) ||
    !Number.isFinite(result.waktu_eksekusi_ms) ||
    !Number.isFinite(result.memori_kb) ||
    (result.pesan_error !== null && typeof result.pesan_error !== 'string') ||
    typeof result.is_hidden !== 'boolean' ||
    (result.actual_output !== null && typeof result.actual_output !== 'string')
  ) {
    throw new Error('Data hasil test case tidak sesuai format yang diharapkan.')
  }

  return {
    testCaseId: String(result.testcase_id),
    verdict: result.status,
    score: result.skor,
    executionTimeMs: result.waktu_eksekusi_ms,
    memoryKb: result.memori_kb,
    hidden: result.is_hidden,
    errorMessage: result.is_hidden ? null : result.pesan_error,
    actualOutput: result.is_hidden ? null : result.actual_output,
  }
}

function readReportResponse(payload, submissionId) {
  if (
    !payload ||
    typeof payload !== 'object' ||
    !isIdentifier(payload.submission_id) ||
    String(payload.submission_id) !== String(submissionId) ||
    !Number.isFinite(payload.skor_total) ||
    !Array.isArray(payload.results)
  ) {
    throw new Error('Respons laporan submission tidak sesuai format yang diharapkan.')
  }

  return {
    submissionId: String(payload.submission_id),
    totalScore: payload.skor_total,
    results: payload.results.map(readResult),
  }
}

export async function getSubmissionReport(submissionId) {
  if (import.meta.env.VITE_USE_MOCK === 'true') {
    return readReportResponse(
      { ...mockSubmissionReport, submission_id: submissionId },
      submissionId,
    )
  }

  const response = await apiRequest(
    `/submissions/${encodeURIComponent(submissionId)}/report`,
  )
  if (!response.ok) {
    throw new Error(`Gagal memuat laporan submission (${response.status}).`)
  }

  return readReportResponse(await response.json(), submissionId)
}
