import { apiRequest, throwApiError, throwMockApiError } from './client.js'

const submissionStatuses = new Set(['QUEUED', 'RUNNING', 'COMPLETED', 'FAILED', 'CANCELLED'])

// Mirrors the API: total_score and max_score are omitted until a submission is graded.
const mockSubmissionHistory = [
  {
    id: 'submission-3',
    assignment_id: 'assignment-2',
    assignment_title: 'Struktur Data',
    status: 'QUEUED',
    is_late: false,
    submitted_at: '2026-10-06T02:00:00Z',
  },
  {
    id: 'submission-2',
    assignment_id: 'assignment-2',
    assignment_title: 'Struktur Data',
    status: 'RUNNING',
    is_late: true,
    submitted_at: '2026-10-05T02:15:00Z',
  },
  {
    id: 'submission-1',
    assignment_id: 'assignment-1',
    assignment_title: 'Dasar Algoritma',
    status: 'COMPLETED',
    total_score: 85,
    max_score: 100,
    is_late: false,
    submitted_at: '2026-10-04T03:30:00Z',
  },
]
const mockCancelledIds = new Set()

function isIdentifier(value) {
  return typeof value === 'string' || typeof value === 'number'
}

function isOptionalScore(value) {
  return value == null || Number.isFinite(value)
}

function readSubmission(submission, formatError) {
  if (
    !submission ||
    typeof submission !== 'object' ||
    !isIdentifier(submission.id) ||
    !isIdentifier(submission.assignment_id) ||
    typeof submission.assignment_title !== 'string' ||
    !submissionStatuses.has(submission.status) ||
    !isOptionalScore(submission.total_score) ||
    !isOptionalScore(submission.max_score) ||
    typeof submission.is_late !== 'boolean' ||
    typeof submission.submitted_at !== 'string' ||
    !Number.isFinite(Date.parse(submission.submitted_at))
  ) {
    throw new Error(formatError)
  }

  return {
    id: String(submission.id),
    assignmentId: String(submission.assignment_id),
    assignmentTitle: submission.assignment_title,
    status: submission.status,
    totalScore: submission.total_score ?? null,
    maxScore: submission.max_score ?? null,
    isLate: submission.is_late,
    submittedAt: submission.submitted_at,
  }
}

function readHistoryResponse(payload, requestedPage, requestedPageSize) {
  const submissions = Array.isArray(payload) ? payload : payload?.items
  if (!Array.isArray(submissions)) {
    throw new Error('Respons riwayat submission tidak sesuai format yang diharapkan.')
  }
  const items = submissions.map((submission) =>
    readSubmission(submission, 'Respons riwayat submission tidak sesuai format yang diharapkan.'),
  )

  const page = Number.isInteger(payload?.page) && payload.page > 0
    ? payload.page
    : requestedPage
  const pageSize = Number.isInteger(payload?.page_size) && payload.page_size > 0
    ? payload.page_size
    : requestedPageSize

  return {
    items,
    page,
    pageSize,
    hasNextPage: typeof payload?.has_next === 'boolean'
      ? payload.has_next
      : submissions.length >= pageSize,
  }
}

function mockSubmissions() {
  return mockSubmissionHistory.map((submission) =>
    mockCancelledIds.has(submission.id) ? { ...submission, status: 'CANCELLED' } : submission,
  )
}

export async function getSubmissionHistory({ page = 1, pageSize = 50 } = {}) {
  if (!Number.isInteger(page) || page < 1 || !Number.isInteger(pageSize) || pageSize < 1) {
    throw new Error('Nomor halaman dan ukuran halaman harus bilangan bulat positif.')
  }

  if (import.meta.env.VITE_USE_MOCK === 'true') {
    return readHistoryResponse(mockSubmissions(), page, pageSize)
  }

  const response = await apiRequest(`/submissions?page=${page}&page_size=${pageSize}`)
  if (!response.ok) {
    await throwApiError(response, `Gagal memuat riwayat submission (${response.status}).`)
  }

  return readHistoryResponse(await response.json(), page, pageSize)
}

// FR-16: only the owner may cancel, and only while the submission is still QUEUED.
export async function cancelSubmission(submissionId) {
  const formatError = 'Respons pembatalan submission tidak sesuai format yang diharapkan.'

  if (import.meta.env.VITE_USE_MOCK === 'true') {
    const submission = mockSubmissions().find((entry) => entry.id === String(submissionId))
    if (!submission) {
      await throwMockApiError('not_found', 'Submission tidak ditemukan.', 404)
    }
    if (submission.status === 'RUNNING') {
      await throwMockApiError(
        'not_cancellable',
        'Submission yang sedang dinilai tidak dapat dibatalkan.',
        409,
      )
    }
    if (submission.status !== 'QUEUED') {
      await throwMockApiError(
        'invalid_status',
        'Hanya submission yang masih antre yang dapat dibatalkan.',
        409,
      )
    }
    mockCancelledIds.add(submission.id)
    return readSubmission({ ...submission, status: 'CANCELLED' }, formatError)
  }

  const response = await apiRequest(`/submissions/${encodeURIComponent(submissionId)}`, {
    method: 'DELETE',
  })
  if (!response.ok) {
    await throwApiError(response, `Gagal membatalkan submission (${response.status}).`, {
      409: 'Submission tidak dapat dibatalkan karena sudah mulai dinilai.',
    })
  }

  const submission = readSubmission(await response.json(), formatError)
  if (submission.id !== String(submissionId)) {
    throw new Error(formatError)
  }
  return submission
}
