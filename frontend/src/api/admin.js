import { apiRequest, throwApiError, throwMockApiError } from './client.js'

// Upper bounds of the FR-14 thresholds, as the API validates them. 0 switches a rule off.
export const rateLimitMaximums = {
  max_active_submissions: 10000,
  min_submission_interval_s: 86400,
  max_submissions_per_assignment: 10000,
}

const roles = new Set(['STUDENT', 'LECTURER', 'ADMIN'])
const mockUsers = [
  { id: 'student-1', name: 'Mahasiswa Demo', email: 'mahasiswa@example.test', role: 'STUDENT' },
  { id: 'lecturer-1', name: 'Dosen Demo', email: 'dosen@example.test', role: 'LECTURER' },
  { id: 'admin-1', name: 'Admin Demo', email: 'admin@example.test', role: 'ADMIN' },
]
const mockAuditLogs = [
  {
    id: 'audit-1',
    user_id: 'student-1',
    submission_id: 'submission-1',
    event_type: 'SANDBOX_VIOLATION',
    syscall_nr: 41,
    detail: 'Percobaan koneksi jaringan ditolak',
    created_at: '2026-10-14T08:00:00Z',
  },
]
const mockQueueStatus = {
  queue_depth: 1,
  workers: { alive: 2, configured: 2 },
  submissions: { queued: 1, running: 1 },
}
const mockRateLimits = {
  max_active_submissions: 1,
  min_submission_interval_s: 30,
  max_submissions_per_assignment: 30,
}

function isCount(value) {
  return Number.isInteger(value) && value >= 0
}

function readPage(payload, parser, message, requestedPage, requestedPageSize) {
  const entries = Array.isArray(payload) ? payload : payload?.items
  if (!Array.isArray(entries)) {
    throw new Error(message)
  }
  const page = Number.isInteger(payload?.page) && payload.page > 0
    ? payload.page
    : requestedPage
  const pageSize = Number.isInteger(payload?.page_size) && payload.page_size > 0
    ? payload.page_size
    : requestedPageSize
  return {
    items: entries.map(parser),
    page,
    pageSize,
    hasNextPage: typeof payload?.has_next === 'boolean'
      ? payload.has_next
      : entries.length >= pageSize,
  }
}

function readUser(user) {
  if (
    !user ||
    (typeof user.id !== 'string' && typeof user.id !== 'number') ||
    typeof user.name !== 'string' ||
    typeof user.email !== 'string' ||
    !roles.has(user.role)
  ) {
    throw new Error('Data pengguna tidak sesuai format yang diharapkan.')
  }
  return { id: String(user.id), name: user.name, email: user.email, role: user.role }
}

function readAuditLog(entry) {
  if (
    !entry ||
    (typeof entry.id !== 'string' && typeof entry.id !== 'number') ||
    (typeof entry.user_id !== 'string' && typeof entry.user_id !== 'number') ||
    typeof entry.event_type !== 'string' ||
    (entry.submission_id != null &&
      typeof entry.submission_id !== 'string' &&
      typeof entry.submission_id !== 'number') ||
    (entry.syscall_nr != null && !Number.isInteger(entry.syscall_nr)) ||
    typeof entry.detail !== 'string' ||
    typeof entry.created_at !== 'string' ||
    !Number.isFinite(Date.parse(entry.created_at))
  ) {
    throw new Error('Data audit log tidak sesuai format yang diharapkan.')
  }
  return {
    id: String(entry.id),
    userId: String(entry.user_id),
    submissionId: entry.submission_id == null ? null : String(entry.submission_id),
    eventType: entry.event_type,
    syscallNumber: entry.syscall_nr ?? null,
    detail: entry.detail,
    createdAt: entry.created_at,
  }
}

async function getPage(path, mockData, parser, message, page, pageSize) {
  if (!Number.isInteger(page) || page < 1 || !Number.isInteger(pageSize) || pageSize < 1) {
    throw new Error('Nomor halaman dan ukuran halaman harus bilangan bulat positif.')
  }
  if (import.meta.env.VITE_USE_MOCK === 'true') {
    return readPage(mockData, parser, message, page, pageSize)
  }
  const response = await apiRequest(`${path}?page=${page}&page_size=${pageSize}`)
  if (!response.ok) {
    await throwApiError(response, `Gagal memuat data administrasi (${response.status}).`)
  }
  return readPage(await response.json(), parser, message, page, pageSize)
}

export async function getAdminUsers(page = 1, pageSize = 50) {
  return getPage(
    '/admin/users',
    mockUsers,
    readUser,
    'Respons daftar pengguna tidak sesuai format.',
    page,
    pageSize,
  )
}

export async function getAdminAuditLogs(page = 1, pageSize = 50) {
  return getPage(
    '/admin/audit-logs',
    mockAuditLogs,
    readAuditLog,
    'Respons daftar audit log tidak sesuai format.',
    page,
    pageSize,
  )
}

export async function updateUserRole(userId, role) {
  if (!roles.has(role)) {
    throw new Error('Peran pengguna tidak valid.')
  }
  if (import.meta.env.VITE_USE_MOCK === 'true') {
    const user = mockUsers.find((entry) => entry.id === userId)
    if (!user) {
      throw new Error('Pengguna tidak ditemukan.')
    }
    return readUser({ ...user, role })
  }

  const response = await apiRequest(`/admin/users/${encodeURIComponent(userId)}/role`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ role }),
  })
  if (!response.ok) {
    await throwApiError(response, `Gagal mengubah peran pengguna (${response.status}).`)
  }
  return readUser(await response.json())
}

function readQueueStatus(payload) {
  if (
    !isCount(payload?.queue_depth) ||
    !isCount(payload.workers?.alive) ||
    !isCount(payload.workers.configured) ||
    !isCount(payload.submissions?.queued) ||
    !isCount(payload.submissions.running)
  ) {
    throw new Error('Respons status antrian tidak sesuai format yang diharapkan.')
  }
  return {
    queueDepth: payload.queue_depth,
    workersAlive: payload.workers.alive,
    workersConfigured: payload.workers.configured,
    queued: payload.submissions.queued,
    running: payload.submissions.running,
  }
}

export async function getQueueStatus() {
  if (import.meta.env.VITE_USE_MOCK === 'true') {
    return readQueueStatus(mockQueueStatus)
  }
  const response = await apiRequest('/admin/queue')
  if (!response.ok) {
    await throwApiError(response, `Gagal memuat status antrian (${response.status}).`)
  }
  return readQueueStatus(await response.json())
}

function isRateLimit(limits, name) {
  return isCount(limits?.[name]) && limits[name] <= rateLimitMaximums[name]
}

// Returns only the three thresholds; other response fields are dropped.
function readRateLimits(payload) {
  const names = Object.keys(rateLimitMaximums)
  if (!names.every((name) => isRateLimit(payload, name))) {
    throw new Error('Respons batas laju tidak sesuai format yang diharapkan.')
  }
  return Object.fromEntries(names.map((name) => [name, payload[name]]))
}

export async function getRateLimits() {
  if (import.meta.env.VITE_USE_MOCK === 'true') {
    return readRateLimits(mockRateLimits)
  }
  const response = await apiRequest('/admin/rate-limits')
  if (!response.ok) {
    await throwApiError(response, `Gagal memuat batas laju (${response.status}).`)
  }
  return readRateLimits(await response.json())
}

// The body holds exactly the three thresholds: the API rejects unknown fields.
export async function updateRateLimits(limits) {
  const names = Object.keys(rateLimitMaximums)
  const body = Object.fromEntries(names.map((name) => [name, limits[name]]))
  if (import.meta.env.VITE_USE_MOCK === 'true') {
    if (!names.every((name) => isRateLimit(body, name))) {
      await throwMockApiError(
        'invalid_settings',
        'Nilai batas laju di luar rentang yang diizinkan.',
      )
    }
    return readRateLimits(Object.assign(mockRateLimits, body))
  }
  const response = await apiRequest('/admin/rate-limits', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  if (!response.ok) {
    await throwApiError(response, `Gagal menyimpan batas laju (${response.status}).`)
  }
  return readRateLimits(await response.json())
}
