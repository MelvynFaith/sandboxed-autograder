import { apiRequest, throwApiError } from './client.js'

const mockSubmissionHistory = [
  {
    id: 'submission-1',
    assignment_id: 'assignment-1',
    assignment_title: 'Dasar Algoritma',
    status: 'COMPLETED',
    total_score: 85,
    is_late: false,
    submitted_at: '2026-10-04T10:30:00+07:00',
  },
  {
    id: 'submission-2',
    assignment_id: 'assignment-2',
    assignment_title: 'Struktur Data',
    status: 'RUNNING',
    total_score: 0,
    is_late: true,
    submitted_at: '2026-10-05T09:15:00+07:00',
  },
]

function isIdentifier(value) {
  return typeof value === 'string' || typeof value === 'number'
}

function readHistoryResponse(payload, requestedPage, requestedPageSize) {
  const submissions = Array.isArray(payload) ? payload : payload?.items
  if (
    !Array.isArray(submissions) ||
    submissions.some(
      (submission) =>
        !submission ||
        typeof submission !== 'object' ||
        !isIdentifier(submission.id) ||
        !isIdentifier(submission.assignment_id) ||
        typeof submission.assignment_title !== 'string' ||
        typeof submission.status !== 'string' ||
        !['QUEUED', 'RUNNING', 'COMPLETED', 'FAILED', 'CANCELLED'].includes(submission.status) ||
        !Number.isFinite(submission.total_score) ||
        typeof submission.is_late !== 'boolean' ||
        typeof submission.submitted_at !== 'string' ||
        !Number.isFinite(Date.parse(submission.submitted_at)),
    )
  ) {
    throw new Error('Respons riwayat submission tidak sesuai format yang diharapkan.')
  }

  const page = Number.isInteger(payload?.page) && payload.page > 0
    ? payload.page
    : requestedPage
  const pageSize = Number.isInteger(payload?.page_size) && payload.page_size > 0
    ? payload.page_size
    : requestedPageSize

  return {
    items: submissions.map((submission) => ({
      id: String(submission.id),
      assignmentId: String(submission.assignment_id),
      assignmentTitle: submission.assignment_title,
      status: submission.status,
      totalScore: submission.total_score,
      isLate: submission.is_late,
      submittedAt: submission.submitted_at,
    })),
    page,
    pageSize,
    hasNextPage: typeof payload?.has_next === 'boolean'
      ? payload.has_next
      : submissions.length >= pageSize,
  }
}

export async function getSubmissionHistory({ page = 1, pageSize = 50 } = {}) {
  if (!Number.isInteger(page) || page < 1 || !Number.isInteger(pageSize) || pageSize < 1) {
    throw new Error('Nomor halaman dan ukuran halaman harus bilangan bulat positif.')
  }

  if (import.meta.env.VITE_USE_MOCK === 'true') {
    return readHistoryResponse(mockSubmissionHistory, page, pageSize)
  }

  const response = await apiRequest(`/submissions?page=${page}&page_size=${pageSize}`)
  if (!response.ok) {
    await throwApiError(response, `Gagal memuat riwayat submission (${response.status}).`)
  }

  return readHistoryResponse(await response.json(), page, pageSize)
}
