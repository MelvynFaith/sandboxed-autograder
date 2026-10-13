import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import App from '../App.jsx'
import { createLecturerAssignment } from '../api/lecturerAssignments.js'
import { createTestCase } from '../api/testcases.js'

vi.mock('../api/lecturerAssignments.js', () => ({
  createLecturerAssignment: vi.fn(),
}))

vi.mock('../api/testcases.js', () => ({
  createTestCase: vi.fn(),
}))

function setUser(role = 'dosen') {
  localStorage.setItem('token', 'lecturer-token')
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

function fillAssignmentForm() {
  fireEvent.change(screen.getByLabelText('Judul'), { target: { value: 'Latihan Algoritma' } })
  fireEvent.change(screen.getByLabelText('Deskripsi'), {
    target: { value: 'Latihan dasar' },
  })
  fireEvent.change(screen.getByLabelText('Deadline'), {
    target: { value: '2026-10-12T23:59' },
  })
  fireEvent.change(screen.getByLabelText('Batas waktu CPU (ms)'), {
    target: { value: '1000' },
  })
  fireEvent.change(screen.getByLabelText('Batas memori (MB)'), {
    target: { value: '128' },
  })
  fireEvent.change(screen.getByLabelText('Jumlah proses maksimum'), {
    target: { value: '4' },
  })
  fireEvent.change(screen.getByLabelText('Waktu maksimum (detik)'), {
    target: { value: '5' },
  })
}

beforeEach(() => {
  localStorage.clear()
  window.history.pushState({}, '', '/lecturer/assignments')
  vi.stubEnv('VITE_USE_MOCK', 'true')
  createLecturerAssignment.mockResolvedValue({
    id: 'assignment-1',
    judul: 'Latihan Algoritma',
    deskripsi: 'Latihan dasar',
    deadline: '2026-10-12T23:59:00.000Z',
  })
  createTestCase.mockResolvedValue({ id: 'testcase-1', assignmentId: 'assignment-1' })
})

afterEach(() => {
  cleanup()
  localStorage.clear()
  vi.unstubAllEnvs()
  vi.clearAllMocks()
})

describe('LecturerAssignmentsPage', () => {
  it('creates an assignment with positive resource limits', async () => {
    setUser()
    render(<App />)
    fillAssignmentForm()
    fireEvent.click(screen.getByRole('button', { name: 'Buat assignment' }))

    expect(await screen.findByRole('heading', { name: 'Tambah test case' })).toBeInTheDocument()
    expect(createLecturerAssignment).toHaveBeenCalledWith({
      judul: 'Latihan Algoritma',
      deskripsi: 'Latihan dasar',
      deadline: new Date('2026-10-12T23:59').toISOString(),
      resource_limit: {
        cpu_time_ms: 1000,
        memory_mb: 128,
        max_proses: 4,
        timeout_s: 5,
      },
    })
  })

  it('rejects non-positive resource limits', async () => {
    setUser()
    render(<App />)
    fillAssignmentForm()
    fireEvent.change(screen.getByLabelText('Batas memori (MB)'), {
      target: { value: '0' },
    })
    fireEvent.submit(screen.getByRole('heading', { name: 'Buat assignment' }).closest('form'))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Semua batas sumber daya harus berupa angka positif.',
    )
    expect(createLecturerAssignment).not.toHaveBeenCalled()
  })

  it('adds a test case with the entity field names', async () => {
    setUser('admin')
    render(<App />)
    fillAssignmentForm()
    fireEvent.click(screen.getByRole('button', { name: 'Buat assignment' }))
    await screen.findByRole('heading', { name: 'Tambah test case' })

    fireEvent.change(screen.getByLabelText('Input'), { target: { value: '2 3' } })
    fireEvent.change(screen.getByLabelText('Keluaran yang diharapkan'), {
      target: { value: '5' },
    })
    fireEvent.change(screen.getByLabelText('Bobot'), { target: { value: '10' } })
    fireEvent.click(screen.getByLabelText('Test case tersembunyi'))
    fireEvent.click(screen.getByRole('button', { name: 'Tambah test case' }))

    expect(await screen.findByText('Test case berhasil ditambahkan.')).toHaveAttribute(
      'role',
      'status',
    )
    expect(createTestCase).toHaveBeenCalledWith('assignment-1', {
      input: '2 3',
      expected_output: '5',
      bobot: 10,
      is_hidden: true,
    })
  })

  it('redirects students away from lecturer assignment management', async () => {
    setUser('mahasiswa')
    render(<App />)

    expect(await screen.findByRole('heading', { name: 'Daftar Assignment' })).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Kelola Assignment' })).not.toBeInTheDocument()
    expect(createLecturerAssignment).not.toHaveBeenCalled()
  })
})
