import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import App from '../App.jsx'
import { getAdminAuditLogs, getAdminUsers, updateUserRole } from '../api/admin.js'

vi.mock('../api/admin.js', () => ({
  getAdminAuditLogs: vi.fn(),
  getAdminUsers: vi.fn(),
  updateUserRole: vi.fn(),
}))

const adminUsers = {
  items: [
    { id: 'student-1', name: 'Mahasiswa Demo', email: 'student@example.test', role: 'STUDENT' },
  ],
}
const auditLogs = {
  items: [
    {
      id: 'audit-1',
      userId: 'student-1',
      submissionId: null,
      eventType: 'SANDBOX_VIOLATION',
      syscallNumber: 41,
      detail: 'Percobaan koneksi ditolak',
      createdAt: '2026-10-14T08:00:00Z',
    },
  ],
}

function setUser(role) {
  localStorage.setItem('token', 'access-token')
  localStorage.setItem('refresh_token', 'refresh-token')
  localStorage.setItem(
    'user',
    JSON.stringify({ id: 'admin-1', nama: 'Admin Demo', email: 'admin@example.test', role }),
  )
}

beforeEach(() => {
  localStorage.clear()
  window.history.pushState({}, '', '/admin')
  vi.stubEnv('VITE_USE_MOCK', 'false')
  getAdminUsers.mockResolvedValue(adminUsers)
  getAdminAuditLogs.mockResolvedValue(auditLogs)
  updateUserRole.mockResolvedValue({ ...adminUsers.items[0], role: 'LECTURER' })
})

afterEach(() => {
  cleanup()
  localStorage.clear()
  vi.unstubAllEnvs()
  vi.clearAllMocks()
})

describe('AdminPage', () => {
  it('shows audit data and allows an administrator to change a user role', async () => {
    setUser('admin')
    render(<App />)

    expect(await screen.findByText('SANDBOX_VIOLATION')).toBeInTheDocument()
    expect(screen.getByText('Percobaan koneksi ditolak')).toBeInTheDocument()
    fireEvent.change(screen.getByLabelText('Ubah peran Mahasiswa Demo'), {
      target: { value: 'LECTURER' },
    })

    expect(updateUserRole).toHaveBeenCalledWith('student-1', 'LECTURER')
    expect(await screen.findByText('Dosen', { selector: 'td' })).toBeInTheDocument()
  })

  it('redirects students away from the administrator page', async () => {
    setUser('mahasiswa')
    render(<App />)

    expect(await screen.findByRole('heading', { name: 'Daftar Assignment' })).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Administrasi' })).not.toBeInTheDocument()
    expect(getAdminUsers).not.toHaveBeenCalled()
  })
})
