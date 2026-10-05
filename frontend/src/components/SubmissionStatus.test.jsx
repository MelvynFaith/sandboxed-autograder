import '@testing-library/jest-dom/vitest'
import { act, cleanup, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { subscribeToSubmissionStatus } from '../api/submissionStatus.js'
import SubmissionStatus from './SubmissionStatus.jsx'

vi.mock('../api/submissionStatus.js', () => ({
  subscribeToSubmissionStatus: vi.fn(),
}))

const statuses = [
  ['queued', 'Antre'],
  ['running', 'Sedang berjalan'],
  ['completed', 'Selesai'],
  ['error', 'Error'],
  ['timeout', 'Waktu habis'],
]

let handlers

beforeEach(() => {
  handlers = null
  subscribeToSubmissionStatus.mockImplementation((_id, _token, callbacks) => {
    handlers = callbacks
    callbacks.onStatus({ status: 'queued', current_test: 0, total_tests: 3 })
    return vi.fn()
  })
})

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

describe('SubmissionStatus', () => {
  it.each(statuses)('shows the %s status badge', (status, label) => {
    render(<SubmissionStatus submissionId="submission-1" token="token-1" />)

    act(() => handlers.onStatus({ status, current_test: 1, total_tests: 3 }))

    expect(screen.getByTestId('submission-status-badge')).toHaveTextContent(label)
  })

  it('shows test progress', () => {
    render(<SubmissionStatus submissionId="submission-1" token="token-1" />)

    act(() => handlers.onStatus({ status: 'running', current_test: 2, total_tests: 4 }))

    expect(screen.getByText('Test 2 dari 4')).toBeInTheDocument()
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '2')
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuemax', '4')
  })

  it('explains when polling is used after the WebSocket disconnects', () => {
    render(<SubmissionStatus submissionId="submission-1" token="token-1" />)

    act(() =>
      handlers.onFallback(
        'Koneksi real-time terputus. Status diperbarui melalui polling setiap 3 detik.',
      ),
    )

    expect(screen.getByRole('status')).toHaveTextContent(
      'Koneksi real-time terputus. Status diperbarui melalui polling setiap 3 detik.',
    )
  })
})
