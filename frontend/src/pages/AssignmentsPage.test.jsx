import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import App from '../App.jsx'

vi.mock('../api/submissionStatus.js', () => ({
  subscribeToSubmissionStatus: vi.fn((_submissionId, { onStatus }) => {
    onStatus({ status: 'QUEUED', current_test: 0, total_tests: 0 })
    return vi.fn()
  }),
}))

function setUser(role) {
  localStorage.setItem('token', 'test-token')
  localStorage.setItem('refresh_token', 'test-refresh-token')
  localStorage.setItem(
    'user',
    JSON.stringify({
      id: role === 'mahasiswa' ? 'mock-student' : 'user-1',
      nama: 'Test User',
      email: 'test@example.test',
      role,
    }),
  )
}

beforeAll(async () => {
  await Promise.all([
    import('./assignment/AssignmentLayout.jsx'),
    import('./assignment/ProblemTab.jsx'),
    import('../components/AssignmentDescription.jsx'),
  ])
})

beforeEach(() => {
  localStorage.clear()
  window.history.pushState({}, '', '/assignments')
  vi.stubEnv('VITE_USE_MOCK', 'true')
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date('2026-10-13T12:00:00+07:00'))
})

afterEach(() => {
  cleanup()
  localStorage.clear()
  vi.useRealTimers()
  vi.unstubAllEnvs()
  vi.unstubAllGlobals()
  vi.clearAllMocks()
})

describe('student assignment list', () => {
  it('lists each course as a table of assignments with deadlines and the latest submission', async () => {
    setUser('mahasiswa')
    render(<App />)

    const table = await screen.findByRole('table', { name: 'Assignment pada Pemrograman Dasar' })
    expect(within(table).getAllByRole('columnheader').map((cell) => cell.textContent)).toEqual([
      'Assignment',
      'Deadline (WIB)',
      'Batas terlambat (WIB)',
      'Submission terakhir',
    ])
    const rows = within(table).getAllByRole('row').slice(1)
    expect(rows).toHaveLength(2)

    const first = within(rows[0])
    expect(first.getByRole('link', { name: 'Dasar Algoritma' })).toHaveAttribute(
      'href',
      '/assignments/assignment-1',
    )
    expect(first.getByText('12 Okt 2026, 23.59')).toBeInTheDocument()
    expect(first.getByText('Masa terlambat')).toBeInTheDocument()
    expect(await first.findByText('Selesai')).toBeInTheDocument()
    expect(first.getByText('85 / 100')).toBeInTheDocument()

    const second = within(rows[1])
    expect(second.queryByText('Masa terlambat')).not.toBeInTheDocument()
    expect(await second.findByText('Antre')).toBeInTheDocument()
  })

  it('keeps the statement, the upload and the submissions off the list', async () => {
    setUser('mahasiswa')
    render(<App />)

    await screen.findByRole('link', { name: 'Dasar Algoritma' })
    expect(screen.queryByText(/Buat program Python/)).not.toBeInTheDocument()
    expect(screen.queryByLabelText(/Kode Python/)).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Upload|Kirim/ })).not.toBeInTheDocument()
  })

  it('opens the problem page from the list', async () => {
    setUser('mahasiswa')
    render(<App />)

    fireEvent.click(await screen.findByRole('link', { name: 'Dasar Algoritma' }))

    expect(await screen.findByRole('heading', { level: 1, name: 'Dasar Algoritma' })).toBeInTheDocument()
    expect(window.location.pathname).toBe('/assignments/assignment-1')
  })

  it('shows lecturers the plain list without links to the student pages', async () => {
    setUser('dosen')
    render(<App />)

    expect(await screen.findByRole('heading', { name: 'Dasar Algoritma' })).toBeInTheDocument()
    expect(screen.getByText(/Buat program Python/)).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Dasar Algoritma' })).not.toBeInTheDocument()
    expect(screen.queryByLabelText(/Kode Python/)).not.toBeInTheDocument()
  })
})
