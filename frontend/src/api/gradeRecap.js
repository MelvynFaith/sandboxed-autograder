import { apiRequest, throwApiError, throwMockApiError } from './client.js'

const submissionStatuses = new Set(['QUEUED', 'RUNNING', 'COMPLETED', 'FAILED', 'CANCELLED'])

const mockRecap = {
  assignment_id: 1,
  assignment_title: 'Dasar Algoritma',
  items: [
    {
      user_id: 10,
      name: 'Ayu Lestari',
      email: 'ayu@example.test',
      submission_count: 2,
      latest_submission: {
        id: 101,
        status: 'COMPLETED',
        total_score: 85,
        max_score: 100,
        is_late: false,
        stale_grade: true,
        submitted_at: '2026-10-04T03:30:00Z',
      },
    },
    {
      user_id: 11,
      name: 'Budi Santoso',
      email: 'budi@example.test',
      submission_count: 1,
      latest_submission: {
        id: 102,
        status: 'QUEUED',
        is_late: true,
        stale_grade: false,
        submitted_at: '2026-10-05T02:15:00Z',
      },
    },
    {
      user_id: 12,
      name: 'Citra Dewi',
      email: 'citra@example.test',
      submission_count: 0,
      latest_submission: null,
    },
  ],
  page: 1,
  page_size: 50,
  has_next: false,
}

function isIdentifier(value) {
  return typeof value === 'string' || typeof value === 'number'
}

function isOptionalScore(value) {
  return value == null || Number.isFinite(value)
}

function readLatest(latest, formatError) {
  if (latest === null) {
    return null
  }
  if (
    !latest ||
    typeof latest !== 'object' ||
    !isIdentifier(latest.id) ||
    !submissionStatuses.has(latest.status) ||
    !isOptionalScore(latest.total_score) ||
    !isOptionalScore(latest.max_score) ||
    typeof latest.is_late !== 'boolean' ||
    typeof latest.stale_grade !== 'boolean' ||
    typeof latest.submitted_at !== 'string' ||
    !Number.isFinite(Date.parse(latest.submitted_at))
  ) {
    throw new Error(formatError)
  }

  return {
    id: String(latest.id),
    status: latest.status,
    totalScore: latest.total_score ?? null,
    maxScore: latest.max_score ?? null,
    isLate: latest.is_late,
    staleGrade: latest.stale_grade,
    submittedAt: latest.submitted_at,
  }
}

function readRecapRow(row, formatError) {
  if (
    !row ||
    typeof row !== 'object' ||
    !isIdentifier(row.user_id) ||
    typeof row.name !== 'string' ||
    typeof row.email !== 'string' ||
    !Number.isInteger(row.submission_count) ||
    row.submission_count < 0 ||
    row.latest_submission === undefined
  ) {
    throw new Error(formatError)
  }

  return {
    userId: String(row.user_id),
    name: row.name,
    email: row.email,
    submissionCount: row.submission_count,
    latestSubmission: readLatest(row.latest_submission, formatError),
  }
}

function readRecap(payload, assignmentId, page, pageSize) {
  const formatError = 'Respons rekap nilai tidak sesuai format yang diharapkan.'
  if (
    !payload ||
    typeof payload !== 'object' ||
    !isIdentifier(payload.assignment_id) ||
    String(payload.assignment_id) !== String(assignmentId) ||
    typeof payload.assignment_title !== 'string' ||
    !Array.isArray(payload.items)
  ) {
    throw new Error(formatError)
  }

  return {
    assignmentId: String(payload.assignment_id),
    assignmentTitle: payload.assignment_title,
    items: payload.items.map((row) => readRecapRow(row, formatError)),
    page: Number.isInteger(payload.page) && payload.page > 0 ? payload.page : page,
    pageSize:
      Number.isInteger(payload.page_size) && payload.page_size > 0 ? payload.page_size : pageSize,
    hasNextPage:
      typeof payload.has_next === 'boolean' ? payload.has_next : payload.items.length >= pageSize,
  }
}

// FR-11: one row per enrolled student, including students with no submission.
export async function getAssignmentRecap(assignmentId, { page = 1, pageSize = 50 } = {}) {
  if (!Number.isInteger(page) || page < 1 || !Number.isInteger(pageSize) || pageSize < 1) {
    throw new Error('Nomor halaman dan ukuran halaman harus bilangan bulat positif.')
  }

  if (import.meta.env.VITE_USE_MOCK === 'true') {
    return readRecap({ ...mockRecap, assignment_id: assignmentId }, assignmentId, page, pageSize)
  }

  const response = await apiRequest(
    `/assignments/${encodeURIComponent(assignmentId)}/results?page=${page}&page_size=${pageSize}`,
  )
  if (!response.ok) {
    await throwApiError(response, `Gagal memuat rekap nilai (${response.status}).`, {
      404: 'Assignment tidak ditemukan.',
    })
  }

  return readRecap(await response.json(), assignmentId, page, pageSize)
}

function csvFilename(response, assignmentId) {
  const match = /filename="?([^";]+)"?/i.exec(response.headers?.get('Content-Disposition') ?? '')
  // Only a plain file name is used; anything with a path separator falls back to the default.
  return match && !/[\\/]/.test(match[1]) ? match[1] : `rekap-assignment-${assignmentId}.csv`
}

function saveBlob(blob, filename) {
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.append(link)
  link.click()
  link.remove()
  URL.revokeObjectURL(url)
}

// The export needs the Authorization header, so it is fetched and saved as a blob; a plain
// link to the endpoint would be unauthenticated.
export async function downloadRecapCsv(assignmentId) {
  let blob
  let filename = `rekap-assignment-${assignmentId}.csv`

  if (import.meta.env.VITE_USE_MOCK === 'true') {
    const rows = mockRecap.items.map((row) => {
      const latest = row.latest_submission
      return [
        row.name,
        row.email,
        latest?.id ?? '',
        latest?.status ?? '',
        latest?.total_score ?? '',
        latest?.max_score ?? '',
        latest?.is_late ?? '',
        latest?.stale_grade ?? '',
        latest?.submitted_at ?? '',
      ].join(',')
    })
    const header = 'name,email,submission_id,status,total_score,max_score,is_late,stale_grade,submitted_at'
    blob = new Blob([[header, ...rows].join('\r\n'), '\r\n'], { type: 'text/csv;charset=utf-8' })
  } else {
    const response = await apiRequest(`/assignments/${encodeURIComponent(assignmentId)}/results/export`)
    if (!response.ok) {
      await throwApiError(response, `Gagal mengunduh rekap nilai (${response.status}).`, {
        404: 'Assignment tidak ditemukan.',
      })
    }
    blob = await response.blob()
    filename = csvFilename(response, assignmentId)
  }

  saveBlob(blob, filename)
  return filename
}

// FR-17: regrade is allowed only from COMPLETED or FAILED and not once the code is deleted.
export async function regradeSubmission(submissionId) {
  const formatError = 'Respons penilaian ulang tidak sesuai format yang diharapkan.'

  if (import.meta.env.VITE_USE_MOCK === 'true') {
    const row = mockRecap.items.find(
      (entry) => String(entry.latest_submission?.id) === String(submissionId),
    )
    const status = row?.latest_submission?.status
    if (!row) {
      await throwMockApiError('not_found', 'Submission tidak ditemukan.', 404)
    }
    if (status !== 'COMPLETED' && status !== 'FAILED') {
      await throwMockApiError(
        'invalid_status',
        'Hanya submission yang sudah selesai dinilai yang dapat dinilai ulang.',
        409,
      )
    }
    return { id: String(submissionId), status: 'QUEUED' }
  }

  const response = await apiRequest(`/submissions/${encodeURIComponent(submissionId)}/regrade`, {
    method: 'POST',
  })
  if (!response.ok) {
    await throwApiError(response, `Gagal menjadwalkan penilaian ulang (${response.status}).`, {
      404: 'Submission tidak ditemukan.',
      409: 'Submission ini tidak dapat dinilai ulang pada status sekarang.',
      410: 'Berkas kode sudah dihapus karena masa retensi.',
    })
  }

  const payload = await response.json()
  if (
    !payload ||
    typeof payload !== 'object' ||
    String(payload.id) !== String(submissionId) ||
    !submissionStatuses.has(payload.status)
  ) {
    throw new Error(formatError)
  }
  return { id: String(payload.id), status: payload.status }
}

export async function regradeAssignment(assignmentId) {
  const formatError = 'Respons penilaian ulang tidak sesuai format yang diharapkan.'

  if (import.meta.env.VITE_USE_MOCK === 'true') {
    return { assignmentId: String(assignmentId), queued: 1, skipped: 0 }
  }

  const response = await apiRequest(
    `/assignments/${encodeURIComponent(assignmentId)}/regrade`,
    { method: 'POST' },
  )
  if (!response.ok) {
    await throwApiError(response, `Gagal menjadwalkan penilaian ulang (${response.status}).`, {
      404: 'Assignment tidak ditemukan.',
    })
  }

  const payload = await response.json()
  if (
    !payload ||
    typeof payload !== 'object' ||
    !isIdentifier(payload.assignment_id) ||
    !Number.isInteger(payload.queued) ||
    !Number.isInteger(payload.skipped)
  ) {
    throw new Error(formatError)
  }
  return {
    assignmentId: String(payload.assignment_id),
    queued: payload.queued,
    skipped: payload.skipped,
  }
}
