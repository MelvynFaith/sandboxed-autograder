import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import App from '../App.jsx'
import { getSubmissionCode } from '../api/submissionCode.js'
import { getSubmissionReport } from '../api/submissionReport.js'

vi.mock('../api/submissionCode.js', () => ({
  getSubmissionCode: vi.fn(),
}))

vi.mock('../api/submissionReport.js', () => ({
  getSubmissionReport: vi.fn(),
}))

function setUser(role) {
  localStorage.setItem('token', 'code-token')
  localStorage.setItem('refresh_token', 'code-refresh-token')
  localStorage.setItem(
    'user',
    JSON.stringify({ id: 'user-1', nama: 'Test User', email: 'test@example.test', role }),
  )
}

beforeEach(() => {
  localStorage.clear()
  window.history.pushState({}, '', '/submissions/7/code')
  getSubmissionCode.mockResolvedValue({
    submissionId: '7',
    originalFilename: 'solusi.py',
    code: 'a = int(input())\nprint(a * 2)\n',
  })
  getSubmissionReport.mockResolvedValue({
    submissionId: '7',
    totalScore: 40,
    results: [
      {
        testCaseId: '3',
        verdict: 'FAILED',
        score: 0,
        executionTimeMs: 12,
        memoryKb: 2048,
        hidden: true,
        errorMessage: null,
        actualOutput: null,
      },
    ],
  })
})

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

describe('SubmissionCodePage', () => {
  it('shows the code with line numbers for a student', async () => {
    setUser('mahasiswa')
    render(<App />)

    expect(await screen.findByText('solusi.py')).toBeInTheDocument()
    expect(getSubmissionCode).toHaveBeenCalledWith('7')
    expect(screen.getByRole('region', { name: /solusi\.py, 2 baris/ })).toHaveAttribute(
      'tabindex',
      '0',
    )
    const gutter = screen.getByTestId('line-numbers')
    expect(gutter).toHaveAttribute('aria-hidden', 'true')
    expect(gutter.textContent).toBe('12')
    expect(screen.getByText(/print\(a \* 2\)/).closest('pre')).toHaveClass('flex-1')
  })

  it('shows the retention message when the code was deleted (410)', async () => {
    setUser('dosen')
    getSubmissionCode.mockRejectedValue(
      new Error('Berkas kode sudah dihapus karena masa retensi.'),
    )
    render(<App />)

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Berkas kode sudah dihapus karena masa retensi.',
    )
    expect(screen.queryByTestId('line-numbers')).not.toBeInTheDocument()
  })
})

describe('SubmissionReportPage', () => {
  it('shows the report to a lecturer with a link to the code', async () => {
    setUser('dosen')
    window.history.pushState({}, '', '/submissions/7/report')
    render(<App />)

    expect(await screen.findByText(/Test case 3/)).toHaveTextContent('(tersembunyi)')
    expect(getSubmissionReport).toHaveBeenCalledWith('7')
    fireEvent.click(screen.getByRole('link', { name: 'Lihat kode' }))
    expect(await screen.findByText('solusi.py')).toBeInTheDocument()
  })

  it('keeps students off the lecturer report', async () => {
    setUser('mahasiswa')
    window.history.pushState({}, '', '/submissions/7/report')
    render(<App />)

    expect(await screen.findByRole('navigation', { name: 'Navigasi utama' })).toBeInTheDocument()
    expect(getSubmissionReport).not.toHaveBeenCalled()
    expect(window.location.pathname).toBe('/assignments')
  })
})
