import { apiRequest } from './client.js'

export const MAX_FILE_SIZE = 256 * 1024

function readSubmissionResponse(payload) {
  if (
    (typeof payload?.id !== 'string' && typeof payload?.id !== 'number') ||
    (typeof payload?.status !== 'string' && typeof payload?.status !== 'number') ||
    !['QUEUED', 'RUNNING', 'COMPLETED', 'FAILED', 'CANCELLED'].includes(payload.status) ||
    typeof payload?.submitted_at !== 'string'
  ) {
    throw new Error('Respons unggah tidak sesuai format yang diharapkan.')
  }

  return {
    id: String(payload.id),
    status: payload.status.toLowerCase(),
    isLate: payload.is_late === true,
    submittedAt: payload.submitted_at,
  }
}

export async function submitSubmission(assignmentId, file) {
  if (import.meta.env.VITE_USE_MOCK === 'true') {
    return readSubmissionResponse({
      id: `mock-submission-${Date.now()}`,
      status: 'QUEUED',
      is_late: false,
      submitted_at: new Date().toISOString(),
    })
  }

  const body = new FormData()
  body.append('assignment_id', String(assignmentId))
  body.append('file', file)

  const response = await apiRequest('/submissions', {
    method: 'POST',
    body,
  })
  if (!response.ok) {
    throw new Error(`Gagal mengunggah kode (${response.status}). Silakan coba lagi.`)
  }

  return readSubmissionResponse(await response.json())
}
