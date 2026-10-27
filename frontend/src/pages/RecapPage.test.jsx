import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import App from '../App.jsx'
import {
  downloadRecapCsv,
  getAssignmentRecap,
  regradeAssignment,
  regradeSubmission,
} from '../api/gradeRecap.js'

vi.mock('../api/gradeRecap.js', () => ({
  downloadRecapCsv: vi.fn(),
  getAssignmentRecap: vi.fn(),
  regradeAssignment: vi.fn(),
  regradeSubmission: vi.fn(),
}))

const recap = {
  assignmentId: '4',
  assignmentTitle: 'Dasar Algoritma',
  page: 1,
  pageSize: 50,
  hasNextPage: false,
  items: [
    {
      userId: '1',
      name: 'Ayu Lestari',
      email: 'ayu@example.test',
      submissionCount: 2,
      latestSubmission: {
        id: '11',
        status: 'COMPLETED',
        totalScore: 80,
        maxScore: 100,
        isLate: true,
        staleGrade: true,
        submittedAt: '2026-10-04T03:30:00Z',
      },
    },
    {
      userId: '2',
      name: 'Budi Santoso',
      email: 'budi@example.test',
      submissionCount: 1,
      latestSubmission: {
        id: '12',
        status: 'QUEUED',
        totalScore: null,
        maxScore: null,
        isLate: false,
        staleGrade: false,
        submittedAt: '2026-10-05T02:15:00Z',
      },
    },
    {
      userId: '3',
      name: 'Citra Dewi',
      email: 'citra@example.test',
      submissionCount: 0,
      latestSubmission: null,
    },
  ],
}

function setUser(role = 'dosen') {
  localStorage.setItem('token', 'recap-token')
  localStorage.setItem('refresh_token', 'recap-refresh-token')
  localStorage.setItem(
    'user',
    JSON.stringify({ id: 'user-1', nama: 'Test User', email: 'test@example.test', role }),
  )
}

function rowOf(name) {
  return screen.getByRole('row', { name: new RegExp(name) })
}

beforeEach(() => {
  localStorage.clear()
  window.history.pushState({}, '', '/lecturer/assignments/4/recap')
  getAssignmentRecap.mockResolvedValue(recap)
  downloadRecapCsv.mockResolvedValue('rekap-assignment-4.csv')
  regradeSubmission.mockResolvedValue({ id: '11', status: 'QUEUED' })
  regradeAssignment.mockResolvedValue({ assignmentId: '4', queued: 2, skipped: 1 })
})

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

describe('RecapPage', () => {
  it('lists every enrolled student with score, late and stale badges and links', async () => {
    setUser()
    render(<App />)

    expect(await screen.findByText('Ayu Lestari')).toBeInTheDocument()
    expect(getAssignmentRecap).toHaveBeenCalledWith('4', { page: 1, pageSize: 50 })
    expect(screen.getByText('Dasar Algoritma')).toBeInTheDocument()

    const graded = within(rowOf('Ayu Lestari'))
    expect(graded.getByText('80 / 100')).toBeInTheDocument()
    expect(graded.getByText('Terlambat')).toBeInTheDocument()
    expect(graded.getByText('Nilai usang')).toBeInTheDocument()
    expect(graded.getByRole('link', { name: 'Laporan Ayu Lestari' })).toHaveAttribute(
      'href',
      '/submissions/11/report',
    )
    expect(graded.getByRole('link', { name: 'Kode Ayu Lestari' })).toHaveAttribute(
      'href',
      '/submissions/11/code',
    )

    const queued = within(rowOf('Budi Santoso'))
    expect(queued.getByText('Belum dinilai')).toBeInTheDocument()
    expect(queued.queryByText('Terlambat')).not.toBeInTheDocument()
    expect(queued.queryByRole('button', { name: /Nilai ulang/ })).not.toBeInTheDocument()

    const none = within(rowOf('Citra Dewi'))
    expect(none.getByText('Belum mengumpulkan')).toBeInTheDocument()
    expect(none.queryByRole('link')).not.toBeInTheDocument()
  })

  it('downloads the CSV through the API client and reports the file name', async () => {
    setUser('admin')
    render(<App />)

    fireEvent.click(await screen.findByRole('button', { name: 'Unduh CSV' }))

    expect(await screen.findByRole('status')).toHaveTextContent('rekap-assignment-4.csv')
    expect(downloadRecapCsv).toHaveBeenCalledWith('4')
  })

  it('shows an error when the CSV download fails', async () => {
    setUser()
    downloadRecapCsv.mockRejectedValue(new Error('Gagal mengunduh rekap nilai (500).'))
    render(<App />)

    fireEvent.click(await screen.findByRole('button', { name: 'Unduh CSV' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Gagal mengunduh rekap nilai')
  })

  it('regrades one submission and reloads the recap', async () => {
    setUser()
    render(<App />)

    fireEvent.click(await screen.findByRole('button', { name: 'Nilai ulang Ayu Lestari' }))

    expect(await screen.findByRole('status')).toHaveTextContent(
      'Submission Ayu Lestari dijadwalkan untuk dinilai ulang.',
    )
    expect(regradeSubmission).toHaveBeenCalledWith('11')
    await waitFor(() => expect(getAssignmentRecap).toHaveBeenCalledTimes(2))
  })

  it('shows the server message when a single regrade is refused', async () => {
    setUser()
    regradeSubmission.mockRejectedValue(new Error('Berkas kode sudah dihapus karena masa retensi.'))
    render(<App />)

    fireEvent.click(await screen.findByRole('button', { name: 'Nilai ulang Ayu Lestari' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('masa retensi')
  })

  it('asks in the page before regrading the whole assignment', async () => {
    setUser()
    render(<App />)

    fireEvent.click(await screen.findByRole('button', { name: 'Nilai ulang semua' }))
    const confirmation = screen.getByRole('dialog', {
      name: 'Konfirmasi penilaian ulang seluruh assignment',
    })
    const confirmButton = within(confirmation).getByRole('button', { name: 'Ya, nilai ulang semua' })
    const cancelButton = within(confirmation).getByRole('button', { name: 'Batal' })
    expect(cancelButton).toHaveFocus()
    expect(regradeAssignment).not.toHaveBeenCalled()

    fireEvent.keyDown(window, { key: 'Tab' })
    expect(confirmButton).toHaveFocus()
    fireEvent.keyDown(window, { key: 'Tab', shiftKey: true })
    expect(cancelButton).toHaveFocus()
    fireEvent.keyDown(window, { key: 'Escape' })
    expect(regradeAssignment).not.toHaveBeenCalled()
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Nilai ulang semua' })).toHaveFocus()

    fireEvent.click(screen.getByRole('button', { name: 'Nilai ulang semua' }))
    fireEvent.click(screen.getByRole('button', { name: 'Ya, nilai ulang semua' }))

    expect(await screen.findByRole('status')).toHaveTextContent(
      '2 submission dijadwalkan untuk dinilai ulang, 1 dilewati.',
    )
    expect(regradeAssignment).toHaveBeenCalledWith('4')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('shows an explicit error when the recap cannot be loaded', async () => {
    setUser()
    getAssignmentRecap.mockRejectedValue(new Error('Assignment tidak ditemukan.'))
    render(<App />)

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Gagal memuat rekap nilai: Assignment tidak ditemukan.',
    )
    expect(screen.queryByRole('button', { name: 'Unduh CSV' })).not.toBeInTheDocument()
  })

  it('paginates the recap', async () => {
    setUser()
    getAssignmentRecap.mockImplementation(async (_id, { page }) =>
      page === 1
        ? { ...recap, hasNextPage: true }
        : { ...recap, page: 2, items: [recap.items[2]], hasNextPage: false },
    )
    render(<App />)

    fireEvent.click(await screen.findByRole('button', { name: 'Berikutnya' }))

    expect(await screen.findByText('Halaman 2')).toBeInTheDocument()
    expect(getAssignmentRecap).toHaveBeenLastCalledWith('4', { page: 2, pageSize: 50 })
    expect(screen.getByRole('button', { name: 'Berikutnya' })).toBeDisabled()
  })

  it('redirects students away from the recap', async () => {
    setUser('mahasiswa')
    render(<App />)

    expect(await screen.findByRole('navigation', { name: 'Navigasi utama' })).toBeInTheDocument()
    expect(getAssignmentRecap).not.toHaveBeenCalled()
    expect(window.location.pathname).toBe('/assignments')
  })
})
