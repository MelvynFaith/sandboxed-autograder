import { apiRequest, throwApiError } from './client.js'

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

function readPage(payload, parser, message) {
  const entries = Array.isArray(payload) ? payload : payload?.items
  if (!Array.isArray(entries)) {
    throw new Error(message)
  }
  return {
    items: entries.map(parser),
    page: Number.isInteger(payload?.page) ? payload.page : 1,
    pageSize: Number.isInteger(payload?.page_size) ? payload.page_size : 50,
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

async function getPage(path, mockData, parser, message) {
  if (import.meta.env.VITE_USE_MOCK === 'true') {
    return readPage(mockData, parser, message)
  }
  const response = await apiRequest(`${path}?page=1&page_size=50`)
  if (!response.ok) {
    await throwApiError(response, `Gagal memuat data administrasi (${response.status}).`)
  }
  return readPage(await response.json(), parser, message)
}

export async function getAdminUsers() {
  return getPage('/admin/users', mockUsers, readUser, 'Respons daftar pengguna tidak sesuai format.')
}

export async function getAdminAuditLogs() {
  return getPage(
    '/admin/audit-logs',
    mockAuditLogs,
    readAuditLog,
    'Respons daftar audit log tidak sesuai format.',
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
