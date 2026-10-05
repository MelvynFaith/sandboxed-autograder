import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import App from '../App.jsx'
import { getSubmissionHistory } from '../api/submissionHistory.js'
import { getSubmissionReport } from '../api/submissionReport.js'

vi.mock('../api/submissionHistory.js', () => ({
  getSubmissionHistory: vi.fn(),
}))

vi.mock('../api/submissionReport.js', () => ({
  getSubmissionReport: vi.fn(),
}))

const submission = {
  id: 'submission-1',
  assignmentId: 'assignment-1',
  assignmentTitle: 'Dasar Algoritma',
  status: 'completed',
  totalScore: 85,
  submittedAt: '2026-10-04T10:30:00+07:00',
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
  getSubmissionHistory.mockResolvedValue([submission])
  getSubmissionReport.mockResolvedValue({
    submissionId: 'submission-1',
    totalScore: 85,
    results: verdicts.map((verdict, index) => ({
      testCaseId: `testcase-${index + 1}`,
      verdict,
      score: verdict === 'PASSED' ? 25 : 0,
      executionTimeMs: 10 + index,
      memoryKb: 1024 * (index + 1),
      hidden: index === 6,
      errorMessage: `Error ${verdict}`,
      actualOutput: `output ${verdict}`,
    })),
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
    expect(screen.getByText('Skor: 85')).toBeInTheDocument()
    expect(screen.getByText('4 Oktober 2026 pukul 10.30')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Lihat laporan' })).toBeInTheDocument()
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
    expect(within(report).getByText('Error PASSED')).toBeInTheDocument()
    expect(within(report).queryByText('Error SECURITY_VIOLATION')).not.toBeInTheDocument()
    expect(within(report).queryByText('output SECURITY_VIOLATION')).not.toBeInTheDocument()
  })

  it('shows the empty-history state', async () => {
    setUser()
    getSubmissionHistory.mockResolvedValue([])
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
