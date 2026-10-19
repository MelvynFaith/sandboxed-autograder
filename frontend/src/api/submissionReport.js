import { apiRequest, throwApiError } from './client.js'

const verdicts = new Set([
  'PASSED',
  'FAILED',
  'TIMEOUT',
  'MEMORY_LIMIT_EXCEEDED',
  'RUNTIME_ERROR',
  'SYNTAX_ERROR',
  'SECURITY_VIOLATION',
])

// Mirrors the student view: a hidden result carries no testcase_id and only status, score, time
// and memory (FR-03).
const mockSubmissionReport = {
  submission_id: 'submission-1',
  total_score: 85,
  max_score: 100,
  results: [
    {
      testcase_id: 'testcase-1',
      external_id: 'tc_01',
      status: 'PASSED',
      score: 25,
      exec_time_ms: 12,
      memory_kb: 2048,
      error_message: null,
      is_hidden: false,
      actual_output: '42',
    },
    {
      testcase_id: 'testcase-2',
      external_id: 'tc_02',
      status: 'RUNTIME_ERROR',
      score: 0,
      exec_time_ms: 18,
      memory_kb: 4096,
      error_message: 'TypeError: unsupported operand type',
      is_hidden: false,
      actual_output: '',
    },
    {
      status: 'SECURITY_VIOLATION',
      score: 0,
      exec_time_ms: 1,
      memory_kb: 1024,
      is_hidden: true,
    },
    {
      testcase_id: 'testcase-4',
      external_id: 'tc_04',
      status: 'FAILED',
      score: 0,
      exec_time_ms: 14,
      memory_kb: 2048,
      error_message: null,
      is_hidden: false,
      actual_output: '0',
    },
    {
      testcase_id: 'testcase-5',
      external_id: 'tc_05',
      status: 'TIMEOUT',
      score: 0,
      exec_time_ms: 5000,
      memory_kb: 8192,
      error_message: 'Batas waktu terlampaui',
      is_hidden: false,
      actual_output: null,
    },
    {
      testcase_id: 'testcase-6',
      external_id: 'tc_06',
      status: 'MEMORY_LIMIT_EXCEEDED',
      score: 0,
      exec_time_ms: 28,
      memory_kb: 65536,
      error_message: 'Batas memori terlampaui',
      is_hidden: false,
      actual_output: null,
    },
    {
      testcase_id: 'testcase-7',
      external_id: 'tc_07',
      status: 'SYNTAX_ERROR',
      score: 0,
      exec_time_ms: 0,
      memory_kb: 0,
      error_message: 'SyntaxError',
      is_hidden: false,
      actual_output: null,
    },
  ],
}

function isIdentifier(value) {
  return typeof value === 'string' || typeof value === 'number'
}

function readResult(result) {
  const hidden = result?.is_hidden === true
  if (
    !result ||
    typeof result !== 'object' ||
    // A hidden result keeps no testcase_id for a student; a visible one always has it.
    (hidden ? result.testcase_id != null && !isIdentifier(result.testcase_id)
      : !isIdentifier(result.testcase_id)) ||
    (result.external_id != null && typeof result.external_id !== 'string') ||
    !verdicts.has(result.status) ||
    !Number.isFinite(result.score) ||
    !Number.isFinite(result.exec_time_ms) ||
    !Number.isFinite(result.memory_kb) ||
    (result.error_message != null && typeof result.error_message !== 'string') ||
    (result.is_hidden !== undefined && typeof result.is_hidden !== 'boolean') ||
    (result.actual_output != null && typeof result.actual_output !== 'string')
  ) {
    throw new Error('Data hasil test case tidak sesuai format yang diharapkan.')
  }

  // Hidden content is dropped here even if a server mistakenly sent it (FR-03).
  return {
    testCaseId: hidden || result.testcase_id == null ? null : String(result.testcase_id),
    externalId: hidden ? null : result.external_id ?? null,
    verdict: result.status,
    score: result.score,
    executionTimeMs: result.exec_time_ms,
    memoryKb: result.memory_kb,
    hidden,
    errorMessage: hidden ? null : result.error_message ?? null,
    actualOutput: hidden ? null : result.actual_output ?? null,
  }
}

function readReportResponse(payload, submissionId) {
  // total_score and max_score are omitted until the submission has been graded.
  if (
    !payload ||
    typeof payload !== 'object' ||
    !isIdentifier(payload.submission_id) ||
    String(payload.submission_id) !== String(submissionId) ||
    (payload.total_score != null && !Number.isFinite(payload.total_score)) ||
    (payload.max_score != null && !Number.isFinite(payload.max_score)) ||
    !Array.isArray(payload.results)
  ) {
    throw new Error('Respons laporan submission tidak sesuai format yang diharapkan.')
  }

  return {
    submissionId: String(payload.submission_id),
    totalScore: payload.total_score ?? null,
    maxScore: payload.max_score ?? null,
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
    await throwApiError(response, `Gagal memuat laporan submission (${response.status}).`)
  }

  return readReportResponse(await response.json(), submissionId)
}
