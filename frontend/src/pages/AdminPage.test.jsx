import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import App from '../App.jsx'
import {
  getAdminAuditLogs,
  getAdminUsers,
  getQueueStatus,
  getRateLimits,
  updateRateLimits,
  updateUserRole,
} from '../api/admin.js'

vi.mock('../api/admin.js', async (importOriginal) => ({
  ...(await importOriginal()),
  getAdminAuditLogs: vi.fn(),
  getAdminUsers: vi.fn(),
  getQueueStatus: vi.fn(),
  getRateLimits: vi.fn(),
  updateRateLimits: vi.fn(),
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
const queueStatus = { queueDepth: 3, workersAlive: 1, workersConfigured: 2, queued: 3, running: 1 }
const rateLimits = {
  max_active_submissions: 1,
  min_submission_interval_s: 30,
  max_submissions_per_assignment: 30,
}
const notFound = () => new Error('Endpoint tidak ditemukan.')

function setUser(role) {
  localStorage.setItem('token', 'access-token')
  localStorage.setItem('refresh_token', 'refresh-token')
  localStorage.setItem(
    'user',
    JSON.stringify({ id: 'admin-1', nama: 'Admin Demo', email: 'admin@example.test', role }),
  )
}

async function rateLimitPanel() {
  return within(await screen.findByRole('region', { name: 'Batas laju submission' }))
}

beforeEach(() => {
  localStorage.clear()
  window.history.pushState({}, '', '/admin')
  vi.stubEnv('VITE_USE_MOCK', 'false')
  getAdminUsers.mockResolvedValue(adminUsers)
  getAdminAuditLogs.mockResolvedValue(auditLogs)
  getQueueStatus.mockResolvedValue(queueStatus)
  getRateLimits.mockResolvedValue(rateLimits)
  updateRateLimits.mockImplementation(async (limits) => limits)
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
    expect(screen.getByText('Peran Mahasiswa Demo diubah menjadi Dosen.')).toHaveAttribute(
      'role',
      'status',
    )
  })

  it('keeps user management working when every other admin endpoint fails', async () => {
    setUser('admin')
    getAdminAuditLogs.mockRejectedValue(notFound())
    getQueueStatus.mockRejectedValue(notFound())
    getRateLimits.mockRejectedValue(notFound())
    render(<App />)

    for (const subject of ['audit log', 'status antrian', 'batas laju']) {
      expect(
        await screen.findByText(`Gagal memuat ${subject}: Endpoint tidak ditemukan.`),
      ).toHaveAttribute('role', 'alert')
    }
    fireEvent.change(await screen.findByLabelText('Ubah peran Mahasiswa Demo'), {
      target: { value: 'LECTURER' },
    })

    expect(updateUserRole).toHaveBeenCalledWith('student-1', 'LECTURER')
    expect(await screen.findByText('Dosen', { selector: 'td' })).toBeInTheDocument()
  })

  it('shows the other panels when the user list fails, and retries only that list', async () => {
    setUser('admin')
    getAdminUsers.mockRejectedValueOnce(notFound())
    render(<App />)

    expect(
      await screen.findByText('Gagal memuat pengguna: Endpoint tidak ditemukan.'),
    ).toHaveAttribute('role', 'alert')
    expect(await screen.findByText('SANDBOX_VIOLATION')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Coba lagi memuat pengguna' }))

    expect(await screen.findByText('Mahasiswa Demo')).toBeInTheDocument()
    expect(getAdminUsers).toHaveBeenCalledTimes(2)
    expect(getAdminAuditLogs).toHaveBeenCalledTimes(1)
  })

  it('shows the queue and worker status and reloads it', async () => {
    setUser('admin')
    render(<App />)

    const queue = within(await screen.findByRole('region', { name: 'Antrian dan worker' }))
    expect(await queue.findByText('1 dari 2')).toBeInTheDocument()
    expect(queue.getByText('Pekerjaan di antrian').nextElementSibling).toHaveTextContent('3')
    expect(queue.getByText('Submission sedang dinilai (RUNNING)').nextElementSibling)
      .toHaveTextContent('1')
    expect(queue.getByText(/Sebagian worker tidak berjalan/)).toBeInTheDocument()

    getQueueStatus.mockResolvedValueOnce({ ...queueStatus, workersAlive: 2, queueDepth: 0 })
    fireEvent.click(queue.getByRole('button', { name: 'Muat ulang status antrian' }))

    expect(await queue.findByText('2 dari 2')).toBeInTheDocument()
    expect(queue.queryByText(/Sebagian worker tidak berjalan/)).not.toBeInTheDocument()
    expect(getQueueStatus).toHaveBeenCalledTimes(2)
  })

  it('saves rate limits, where 0 switches a rule off', async () => {
    setUser('admin')
    render(<App />)

    const panel = await rateLimitPanel()
    const interval = await panel.findByLabelText('Jeda minimum antarsubmission (detik)')
    expect(interval).toHaveValue(30)
    expect(interval).toHaveAccessibleDescription(/Isi 0 untuk mematikan aturan ini\./)
    fireEvent.change(panel.getByLabelText('Submission aktif maksimum'), {
      target: { value: '0' },
    })
    fireEvent.change(interval, { target: { value: '60' } })
    fireEvent.click(panel.getByRole('button', { name: 'Simpan batas laju' }))

    expect(await panel.findByText('Batas laju berhasil disimpan.')).toHaveAttribute(
      'role',
      'status',
    )
    expect(updateRateLimits).toHaveBeenCalledWith({
      max_active_submissions: 0,
      min_submission_interval_s: 60,
      max_submissions_per_assignment: 30,
    })
  })

  it.each([
    ['Submission aktif maksimum', '-1', 10000],
    ['Jeda minimum antarsubmission (detik)', '86401', 86400],
    ['Submission maksimum per assignment', '10001', 10000],
    ['Submission maksimum per assignment', '2.5', 10000],
    ['Submission maksimum per assignment', '', 10000],
  ])('rejects %s = "%s" before calling the API', async (label, value, maximum) => {
    setUser('admin')
    render(<App />)

    const panel = await rateLimitPanel()
    const input = await panel.findByLabelText(label)
    fireEvent.change(input, { target: { value } })
    fireEvent.click(panel.getByRole('button', { name: 'Simpan batas laju' }))

    expect(await panel.findByRole('alert')).toHaveTextContent(
      `${label} harus bilangan bulat 0 sampai ${maximum}.`,
    )
    expect(input).toHaveAttribute('aria-invalid', 'true')
    expect(input).toHaveFocus()
    expect(updateRateLimits).not.toHaveBeenCalled()
  })

  it('shows the API message when saving rate limits fails', async () => {
    setUser('admin')
    updateRateLimits.mockRejectedValueOnce(new Error('Nilai batas laju tidak valid.'))
    render(<App />)

    const panel = await rateLimitPanel()
    await panel.findByLabelText('Submission aktif maksimum')
    fireEvent.click(panel.getByRole('button', { name: 'Simpan batas laju' }))

    expect(await panel.findByRole('alert')).toHaveTextContent('Nilai batas laju tidak valid.')
    await waitFor(() =>
      expect(panel.getByRole('button', { name: 'Simpan batas laju' })).toBeEnabled(),
    )
  })

  it('redirects students away from the administrator page', async () => {
    setUser('mahasiswa')
    render(<App />)

    expect(await screen.findByRole('heading', { name: 'Daftar Assignment' })).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Administrasi' })).not.toBeInTheDocument()
    expect(getAdminUsers).not.toHaveBeenCalled()
  })

  it('paginates user and audit-log lists independently', async () => {
    setUser('admin')
    getAdminUsers.mockImplementation(async (page) =>
      page === 1
        ? { ...adminUsers, hasNextPage: true }
        : {
            items: [
              {
                id: 'student-2',
                name: 'Mahasiswa Halaman 2',
                email: 'second@example.test',
                role: 'STUDENT',
              },
            ],
            hasNextPage: false,
          },
    )
    getAdminAuditLogs.mockImplementation(async (page) =>
      page === 1
        ? { ...auditLogs, hasNextPage: true }
        : {
            items: [{ ...auditLogs.items[0], id: 'audit-2', eventType: 'PAGE_TWO_EVENT' }],
            hasNextPage: false,
          },
    )
    render(<App />)

    expect(await screen.findByText('Mahasiswa Demo')).toBeInTheDocument()
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Pengguna berikutnya' })).toBeEnabled(),
    )
    fireEvent.click(screen.getByRole('button', { name: 'Pengguna berikutnya' }))
    expect(await screen.findByText('Mahasiswa Halaman 2')).toBeInTheDocument()
    expect(getAdminUsers).toHaveBeenLastCalledWith(2, 50)
    expect(getAdminAuditLogs).toHaveBeenLastCalledWith(1, 50)

    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Audit log berikutnya' })).toBeEnabled(),
    )
    fireEvent.click(screen.getByRole('button', { name: 'Audit log berikutnya' }))
    expect(await screen.findByText('PAGE_TWO_EVENT')).toBeInTheDocument()
    expect(getAdminAuditLogs).toHaveBeenLastCalledWith(2, 50)
    expect(getAdminUsers).toHaveBeenLastCalledWith(2, 50)
    expect(screen.getByRole('button', { name: 'Audit log sebelumnya' })).toBeEnabled()
  })
})
