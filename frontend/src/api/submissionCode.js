import { apiRequest, throwApiError, throwMockApiError } from './client.js'

const formatError = 'Respons kode submission tidak sesuai format yang diharapkan.'

const mockCode = `def solve(n):
    return n * 2


print(solve(int(input())))
`

function isIdentifier(value) {
  return typeof value === 'string' || typeof value === 'number'
}

// FR-20: the owner, the lecturer of the course and an admin may read the submitted source.
// The API answers 410 once the retention period (NFR-13) has removed the file.
export async function getSubmissionCode(submissionId) {
  let payload

  if (import.meta.env.VITE_USE_MOCK === 'true') {
    if (String(submissionId).includes('deleted')) {
      await throwMockApiError(
        'code_deleted',
        'Berkas kode sudah dihapus karena masa retensi.',
        410,
      )
    }
    payload = {
      submission_id: submissionId,
      original_filename: 'solusi.py',
      code: mockCode,
    }
  } else {
    const response = await apiRequest(`/submissions/${encodeURIComponent(submissionId)}/code`)
    if (!response.ok) {
      await throwApiError(response, `Gagal memuat kode submission (${response.status}).`, {
        404: 'Submission tidak ditemukan.',
        410: 'Berkas kode sudah dihapus karena masa retensi.',
      })
    }
    payload = await response.json()
  }

  if (
    !payload ||
    typeof payload !== 'object' ||
    !isIdentifier(payload.submission_id) ||
    String(payload.submission_id) !== String(submissionId) ||
    typeof payload.original_filename !== 'string' ||
    typeof payload.code !== 'string'
  ) {
    throw new Error(formatError)
  }

  return {
    submissionId: String(payload.submission_id),
    originalFilename: payload.original_filename,
    code: payload.code,
  }
}
