import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import App from '../App.jsx'
import { cancelSubmission, getSubmissionHistory } from '../api/submissionHistory.js'
import { getSubmissionReport } from '../api/submissionReport.js'

vi.mock('../api/submissionHistory.js', () => ({
  cancelSubmission: vi.fn(),
  getSubmissionHistory: vi.fn(),
}))

vi.mock('../api/submissionReport.js', () => ({
  getSubmissionReport: vi.fn(),
}))

const submission = {
  id: 'submission-1',
  assignmentId: 'assignment-1',
  assignmentTitle: 'Dasar Algoritma',
  status: 'COMPLETED',
  totalScore: 85,
  maxScore: 100,
  isLate: true,
  submittedAt: '2026-10-04T10:30:00+07:00',
}

const queuedSubmission = {
  id: 'submission-2',
  assignmentId: 'assignment-2',
  assignmentTitle: 'Struktur Data',
  status: 'QUEUED',
  totalScore: null,
  maxScore: null,
  isLate: false,
  submittedAt: '2026-10-05T10:30:00+07:00',
}

function pageOf(items) {
  return { items, page: 1, pageSize: 50, hasNextPage: false }
}

const verdicts = [
  'PASSED',
  'FAILED',
  'TIMEOUT',
  'MEMORY_LIMIT_EXCEEDED',
  'RUNTIME_ERROR',
  'SYNTAX_ERROR',
  'SECURITY_VIOLATION',
]

function setUser(role = 'mahasiswa') {
  localStorage.setItem('token', 'history-token')
  localStorage.setItem('refresh_token', 'history-refresh-token')
  localStorage.setItem(
    'user',
    JSON.stringify({
      id: 'user-1',
      nama: 'Test User',
      email: 'test@example.test',
      role,
    }),
  )
}

beforeEach(() => {
  localStorage.clear()
  window.history.pushState({}, '', '/history')
  vi.stubEnv('VITE_USE_MOCK', 'true')
  getSubmissionHistory.mockResolvedValue({
    items: [submission],
    page: 1,
    pageSize: 50,
    hasNextPage: false,
  })
  getSubmissionReport.mockResolvedValue({
    submissionId: 'submission-1',
    totalScore: 85,
    maxScore: 100,
    results: verdicts.map((verdict, index) => {
      const hidden = index === 6
      return {
        // A hidden result has no testcase_id for a student and carries no output.
        testCaseId: hidden ? null : `testcase-${index + 1}`,
        externalId: hidden ? null : `tc_0${index + 1}`,
        verdict,
        score: verdict === 'PASSED' ? 25 : 0,
        executionTimeMs: 10 + index,
        memoryKb: 1024 * (index + 1),
        hidden,
        errorMessage: hidden ? null : `Error ${verdict}`,
        actualOutput: hidden ? null : `output ${verdict}`,
      }
    }),
  })
})

afterEach(() => {
  cleanup()
  localStorage.clear()
  vi.unstubAllEnvs()
  vi.clearAllMocks()
})

describe('HistoryPage', () => {
  it('shows the student submission list with assignment, score, and submission time', async () => {
    setUser()
    render(<App />)

    expect(await screen.findByRole('heading', { name: 'Riwayat Submission' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Dasar Algoritma' })).toBeInTheDocument()
    expect(screen.getByText('Skor: 85 / 100')).toBeInTheDocument()
    expect(screen.getByText('Terlambat')).toBeInTheDocument()
    expect(screen.getByText('4 Oktober 2026 pukul 10.30')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Lihat laporan' })).toBeInTheDocument()
  })

  it('shows an ungraded submission without a score or an error', async () => {
    setUser()
    getSubmissionHistory.mockResolvedValue(pageOf([queuedSubmission]))
    render(<App />)

    expect(await screen.findByRole('heading', { name: 'Struktur Data' })).toBeInTheDocument()
    expect(screen.getByText('Skor: Belum dinilai')).toBeInTheDocument()
    expect(screen.getByText('Antre')).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('does not show the late marker for an on-time submission', async () => {
    setUser()
    getSubmissionHistory.mockResolvedValue({
      items: [{ ...submission, isLate: false }],
      page: 1,
      pageSize: 50,
      hasNextPage: false,
    })
    render(<App />)

    await screen.findByRole('heading', { name: 'Dasar Algoritma' })
    expect(screen.queryByText('Terlambat')).not.toBeInTheDocument()
  })

  it('paginates the submission history', async () => {
    setUser()
    getSubmissionHistory
      .mockResolvedValueOnce({
        items: [submission],
        page: 1,
        pageSize: 50,
        hasNextPage: true,
      })
      .mockResolvedValueOnce({
        items: [{ ...submission, id: 'submission-2', assignmentTitle: 'Struktur Data' }],
        page: 2,
        pageSize: 50,
        hasNextPage: false,
      })
    render(<App />)

    await screen.findByRole('heading', { name: 'Dasar Algoritma' })
    fireEvent.click(screen.getByRole('button', { name: 'Berikutnya' }))

    expect(await screen.findByRole('heading', { name: 'Struktur Data' })).toBeInTheDocument()
    expect(getSubmissionHistory).toHaveBeenLastCalledWith({ page: 2, pageSize: 50 })
    expect(screen.getByRole('button', { name: 'Sebelumnya' })).toBeEnabled()
  })

  it('shows returned error messages and actual output when report visibility metadata is absent', async () => {
    setUser()
    getSubmissionReport.mockResolvedValue({
      submissionId: 'submission-1',
      totalScore: 25,
      results: [
        {
          testCaseId: 'testcase-1',
          verdict: 'FAILED',
          score: 0,
          executionTimeMs: 10,
          memoryKb: 1024,
          hidden: false,
          errorMessage: 'Assertion failed',
          actualOutput: 'incorrect result',
        },
      ],
    })
    render(<App />)

    fireEvent.click(await screen.findByRole('button', { name: 'Lihat laporan' }))

    expect(await screen.findByText('Assertion failed')).toBeInTheDocument()
    expect(screen.getByText('incorrect result')).toBeInTheDocument()
  })

  it('shows seven distinct verdict labels and keeps hidden result details private', async () => {
    setUser()
    render(<App />)

    await screen.findByRole('heading', { name: 'Dasar Algoritma' })
    fireEvent.click(screen.getByRole('button', { name: 'Lihat laporan' }))

    const report = await screen.findByRole('region', {
      name: 'Laporan submission submission-1',
    })
    const badges = within(report).getAllByTestId('verdict-badge')
    expect(badges).toHaveLength(7)
    verdicts.forEach((verdict) => {
      expect(within(report).getByText(verdict, { exact: true })).toBeInTheDocument()
    })
    expect(within(report).getByText('Skor total: 85 / 100')).toBeInTheDocument()
    expect(within(report).getByText('Error PASSED')).toBeInTheDocument()
    expect(within(report).getByText('Test case tc_01')).toBeInTheDocument()
    expect(within(report).queryByText('Error SECURITY_VIOLATION')).not.toBeInTheDocument()
    expect(within(report).queryByText('output SECURITY_VIOLATION')).not.toBeInTheDocument()
  })

  it('shows only status, score, time and memory for a hidden result without a test case id', async () => {
    setUser()
    getSubmissionReport.mockResolvedValue({
      submissionId: 'submission-1',
      totalScore: null,
      maxScore: null,
      results: [
        {
          testCaseId: null,
          externalId: null,
          verdict: 'FAILED',
          score: 0,
          executionTimeMs: 7,
          memoryKb: 512,
          hidden: true,
          errorMessage: null,
          actualOutput: null,
        },
      ],
    })
    render(<App />)

    fireEvent.click(await screen.findByRole('button', { name: 'Lihat laporan' }))

    const report = await screen.findByRole('region', { name: 'Laporan submission submission-1' })
    expect(within(report).getByRole('heading', { name: 'Test case tersembunyi 1' })).toBeInTheDocument()
    expect(within(report).getByText('Skor total: Belum dinilai')).toBeInTheDocument()
    expect(within(report).getByText('7 ms')).toBeInTheDocument()
    expect(within(report).getByText('512 KB')).toBeInTheDocument()
    expect(within(report).queryByText('Pesan error:')).not.toBeInTheDocument()
    expect(within(report).queryByText('Output aktual')).not.toBeInTheDocument()
  })

  it('links each submission to its code view', async () => {
    setUser()
    render(<App />)

    const link = await screen.findByRole('link', { name: 'Lihat kode Dasar Algoritma' })
    expect(link).toHaveAttribute('href', '/submissions/submission-1/code')
  })

  it('offers cancel only for a queued submission', async () => {
    setUser()
    getSubmissionHistory.mockResolvedValue(pageOf([submission, queuedSubmission]))
    render(<App />)

    await screen.findByRole('heading', { name: 'Struktur Data' })
    expect(screen.getAllByRole('button', { name: /^Batalkan/ })).toHaveLength(1)
    expect(
      screen.getByRole('button', { name: 'Batalkan submission Struktur Data' }),
    ).toBeInTheDocument()
  })

  it('confirms in the page before cancelling and marks the submission cancelled', async () => {
    setUser()
    const confirm = vi.spyOn(window, 'confirm')
    getSubmissionHistory.mockResolvedValue(pageOf([queuedSubmission]))
    cancelSubmission.mockResolvedValue({ ...queuedSubmission, status: 'CANCELLED' })
    render(<App />)

    fireEvent.click(await screen.findByRole('button', { name: /^Batalkan/ }))
    const confirmation = screen.getByRole('dialog', {
      name: 'Konfirmasi pembatalan submission Struktur Data',
    })
    const confirmButton = within(confirmation).getByRole('button', { name: 'Ya, batalkan' })
    const cancelButton = within(confirmation).getByRole('button', { name: 'Tidak' })
    expect(cancelButton).toHaveFocus()
    expect(cancelSubmission).not.toHaveBeenCalled()

    fireEvent.keyDown(window, { key: 'Tab' })
    expect(confirmButton).toHaveFocus()
    fireEvent.keyDown(window, { key: 'Tab', shiftKey: true })
    expect(cancelButton).toHaveFocus()
    fireEvent.click(confirmButton)

    expect(await screen.findByRole('status')).toHaveTextContent('Submission berhasil dibatalkan.')
    expect(cancelSubmission).toHaveBeenCalledWith('submission-2')
    expect(screen.getByText('Dibatalkan')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /^Batalkan/ })).not.toBeInTheDocument()
    expect(confirm).not.toHaveBeenCalled()
    confirm.mockRestore()
  })

  it('keeps the submission when the cancel confirmation is dismissed', async () => {
    setUser()
    getSubmissionHistory.mockResolvedValue(pageOf([queuedSubmission]))
    render(<App />)

    fireEvent.click(await screen.findByRole('button', { name: /^Batalkan/ }))
    fireEvent.click(screen.getByRole('button', { name: 'Tidak' }))

    expect(cancelSubmission).not.toHaveBeenCalled()
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: /^Batalkan/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /^Batalkan/ })).toHaveFocus()
  })

  it('dismisses the cancel confirmation with Escape and restores focus', async () => {
    setUser()
    getSubmissionHistory.mockResolvedValue(pageOf([queuedSubmission]))
    render(<App />)

    const trigger = await screen.findByRole('button', {
      name: 'Batalkan submission Struktur Data',
    })
    fireEvent.click(trigger)
    expect(screen.getByRole('dialog')).toBeInTheDocument()

    fireEvent.keyDown(window, { key: 'Escape' })

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(trigger).toHaveFocus()
    expect(cancelSubmission).not.toHaveBeenCalled()
  })

  it('shows the server message and reloads when the submission already started', async () => {
    setUser()
    getSubmissionHistory
      .mockResolvedValueOnce(pageOf([queuedSubmission]))
      .mockResolvedValueOnce(pageOf([{ ...queuedSubmission, status: 'RUNNING' }]))
    cancelSubmission.mockRejectedValue(
      new Error('Submission yang sedang dinilai tidak dapat dibatalkan.'),
    )
    render(<App />)

    fireEvent.click(await screen.findByRole('button', { name: /^Batalkan/ }))
    fireEvent.click(screen.getByRole('button', { name: 'Ya, batalkan' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Submission yang sedang dinilai tidak dapat dibatalkan.',
    )
    expect(await screen.findByText('Sedang berjalan')).toBeInTheDocument()
    expect(getSubmissionHistory).toHaveBeenCalledTimes(2)
    expect(screen.queryByRole('button', { name: /^Batalkan/ })).not.toBeInTheDocument()
    expect(screen.getByRole('alert')).toHaveTextContent('tidak dapat dibatalkan')
  })

  it('shows the empty-history state', async () => {
    setUser()
    getSubmissionHistory.mockResolvedValue({
      items: [],
      page: 1,
      pageSize: 50,
      hasNextPage: false,
    })
    render(<App />)

    expect(await screen.findByText('Belum ada submission.')).toBeInTheDocument()
  })

  it('shows an explicit error when the history request fails', async () => {
    setUser()
    getSubmissionHistory.mockRejectedValue(new Error('Server tidak tersedia.'))
    render(<App />)

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Gagal memuat riwayat submission: Server tidak tersedia.',
    )
  })

  it('redirects non-student users away from student history', async () => {
    setUser('admin')
    render(<App />)

    expect(await screen.findByRole('heading', { name: 'Daftar Assignment' })).toBeInTheDocument()
    expect(getSubmissionHistory).not.toHaveBeenCalled()
  })
})
