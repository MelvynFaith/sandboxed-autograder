import { apiRequest } from './client.js'

const mockSubmissionHistory = [
  {
    id: 'submission-1',
    assignment_id: 'assignment-1',
    assignment_judul: 'Dasar Algoritma',
    status: 'completed',
    skor_total: 85,
    submitted_at: '2026-10-04T10:30:00+07:00',
  },
  {
    id: 'submission-2',
    assignment_id: 'assignment-2',
    assignment_judul: 'Struktur Data',
    status: 'running',
    skor_total: 0,
    submitted_at: '2026-10-05T09:15:00+07:00',
  },
]

function isIdentifier(value) {
  return typeof value === 'string' || typeof value === 'number'
}

function readHistoryResponse(payload) {
  if (
    !Array.isArray(payload) ||
    payload.some(
      (submission) =>
        !submission ||
        typeof submission !== 'object' ||
        !isIdentifier(submission.id) ||
        !isIdentifier(submission.assignment_id) ||
        typeof submission.assignment_judul !== 'string' ||
        typeof submission.status !== 'string' ||
        !Number.isFinite(submission.skor_total) ||
        typeof submission.submitted_at !== 'string' ||
        !Number.isFinite(Date.parse(submission.submitted_at)),
    )
  ) {
    throw new Error('Respons riwayat submission tidak sesuai format yang diharapkan.')
  }

  return payload.map((submission) => ({
    id: String(submission.id),
    assignmentId: String(submission.assignment_id),
    assignmentTitle: submission.assignment_judul,
    status: submission.status,
    totalScore: submission.skor_total,
    submittedAt: submission.submitted_at,
  }))
}

export async function getSubmissionHistory() {
  if (import.meta.env.VITE_USE_MOCK === 'true') {
    return readHistoryResponse(mockSubmissionHistory)
  }

  const response = await apiRequest('/submissions')
  if (!response.ok) {
    throw new Error(`Gagal memuat riwayat submission (${response.status}).`)
  }

  return readHistoryResponse(await response.json())
}
