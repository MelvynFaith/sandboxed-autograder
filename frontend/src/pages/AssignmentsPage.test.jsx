import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import App from '../App.jsx'
import { submitSubmission } from '../api/submissions.js'

vi.mock('../api/submissions.js', async (importOriginal) => ({
  ...(await importOriginal()),
  submitSubmission: vi.fn(),
}))

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

function jsonResponse(payload) {
  return {
    ok: true,
    status: 200,
    json: async () => payload,
  }
}

beforeEach(() => {
  localStorage.clear()
  window.history.pushState({}, '', '/assignments')
  vi.stubEnv('VITE_USE_MOCK', 'true')
  submitSubmission.mockResolvedValue({
    id: 'submission-1',
    status: 'QUEUED',
    isLate: false,
    submittedAt: '2026-10-12T10:00:00Z',
  })
})

afterEach(() => {
  cleanup()
  localStorage.clear()
  vi.unstubAllEnvs()
  vi.unstubAllGlobals()
  vi.clearAllMocks()
})

describe('student assignment submissions', () => {
  it('shows assignments from the API', async () => {
    setUser('mahasiswa')
    render(<App />)

    expect(await screen.findByRole('heading', { name: 'Dasar Algoritma' })).toBeInTheDocument()
    expect(screen.getByText(/Buat program Python/)).toBeInTheDocument()
    expect(screen.getAllByText(/Deadline:/)).toHaveLength(2)
    expect(screen.getByText('12 Oktober 2026 pukul 23.59')).toBeInTheDocument()
  })

  it('uploads a valid Python file and shows success', async () => {
    setUser('mahasiswa')
    render(<App />)
    await screen.findByRole('heading', { name: 'Dasar Algoritma' })

    const file = new File(['print("hello")'], 'solution.py', { type: 'text/x-python' })
    fireEvent.change(screen.getByLabelText('Kode Python untuk Dasar Algoritma'), {
      target: { files: [file] },
    })
    fireEvent.click(screen.getAllByRole('button', { name: 'Upload kode Python' })[0])

    expect(await screen.findByText('Kode berhasil diunggah.')).toBeInTheDocument()
    expect(await screen.findByTestId('submission-status-badge')).toHaveTextContent('Antre')
  })

  it('hides single-page pagination and clears an earlier status when a new upload is rate limited', async () => {
    setUser('mahasiswa')
    vi.stubEnv('VITE_USE_MOCK', 'false')
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse({
        items: [{ id: 1, code: 'CS101', name: 'Algoritma', term: '2026' }],
        page: 1,
        page_size: 10,
        has_next: false,
      }))
      .mockResolvedValueOnce(jsonResponse({
        items: [{
          id: 4,
          course_id: 1,
          title: 'Tugas Algoritma',
          description: 'Deskripsi',
          deadline_at: '2026-10-12T16:59:00Z',
          late_until_at: '2026-10-13T16:59:00Z',
        }],
        page: 1,
        page_size: 10,
        has_next: false,
      }))
    vi.stubGlobal('fetch', fetchMock)
    submitSubmission
      .mockResolvedValueOnce({
        id: 'submission-1',
        status: 'QUEUED',
        isLate: false,
        submittedAt: '2026-10-12T10:00:00Z',
      })
      .mockRejectedValueOnce(new Error('Tunggu setidaknya 14 detik sebelum mengirim submission berikutnya.'))
    render(<App />)

    expect(await screen.findByRole('heading', { name: 'Tugas Algoritma' })).toBeInTheDocument()
    expect(screen.queryByRole('navigation', { name: /Halaman/ })).not.toBeInTheDocument()

    const fileInput = screen.getByLabelText('Kode Python untuk Tugas Algoritma')
    const firstFile = new File(['print(1)'], 'first.py', { type: 'text/x-python' })
    fireEvent.change(fileInput, { target: { files: [firstFile] } })
    fireEvent.click(screen.getByRole('button', { name: 'Upload kode Python' }))
    expect(await screen.findByRole('region', { name: 'Status submission submission-1' }))
      .toBeInTheDocument()

    const secondFile = new File(['print(2)'], 'second.py', { type: 'text/x-python' })
    fireEvent.change(fileInput, { target: { files: [secondFile] } })
    fireEvent.click(screen.getByRole('button', { name: 'Upload kode Python' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Tunggu setidaknya 14 detik')
    expect(screen.queryByRole('region', { name: 'Status submission submission-1' }))
      .not.toBeInTheDocument()
  })

  it('rejects files that do not have a Python extension', async () => {
    setUser('mahasiswa')
    render(<App />)
    await screen.findByRole('heading', { name: 'Dasar Algoritma' })

    const file = new File(['not python'], 'solution.txt', { type: 'text/plain' })
    fireEvent.change(screen.getByLabelText('Kode Python untuk Dasar Algoritma'), {
      target: { files: [file] },
    })

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Pilih berkas dengan ekstensi .py.',
    )
    expect(screen.getAllByRole('button', { name: 'Upload kode Python' })[0]).toBeDisabled()
  })

  it('rejects Python files larger than 256 KiB', async () => {
    setUser('mahasiswa')
    render(<App />)
    await screen.findByRole('heading', { name: 'Dasar Algoritma' })

    const file = new File([new Uint8Array(256 * 1024 + 1)], 'large.py')
    fireEvent.change(screen.getByLabelText('Kode Python untuk Dasar Algoritma'), {
      target: { files: [file] },
    })

    expect(await screen.findByRole('alert')).toHaveTextContent('Ukuran berkas maksimal 256 KB.')
    expect(screen.getAllByRole('button', { name: 'Upload kode Python' })[0]).toBeDisabled()
  })

  it('does not show upload controls to non-student roles', async () => {
    setUser('dosen')
    render(<App />)

    expect(await screen.findByRole('heading', { name: 'Dasar Algoritma' })).toBeInTheDocument()
    expect(screen.queryByLabelText('Kode Python untuk Dasar Algoritma')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Upload kode Python' })).not.toBeInTheDocument()
  })
})
