import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import App from '../App.jsx'
import {
  createLecturerAssignment,
  updateLecturerAssignment,
  updateResourceLimit,
} from '../api/lecturerAssignments.js'
import { createTestCase, getTestCases } from '../api/testcases.js'

vi.mock('../api/lecturerAssignments.js', () => ({
  createLecturerAssignment: vi.fn(),
  deleteLecturerAssignment: vi.fn(),
  getResourceLimit: vi.fn(),
  updateLecturerAssignment: vi.fn(),
  updateResourceLimit: vi.fn(),
}))

vi.mock('../api/testcases.js', () => ({
  createTestCase: vi.fn(),
  deleteTestCase: vi.fn(),
  getTestCases: vi.fn(),
  updateTestCase: vi.fn(),
}))

function setUser(role = 'dosen') {
  localStorage.setItem('token', 'lecturer-token')
  localStorage.setItem('refresh_token', 'lecturer-refresh-token')
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

async function fillAssignmentForm() {
  await screen.findByLabelText('Judul')
  fireEvent.change(screen.getByLabelText('Judul'), { target: { value: 'Latihan Algoritma' } })
  fireEvent.change(screen.getByLabelText('Deskripsi'), {
    target: { value: 'Latihan dasar' },
  })
  fireEvent.change(screen.getByLabelText('Deadline'), {
    target: { value: '2026-10-12T23:59' },
  })
  fireEvent.change(screen.getByLabelText('Batas akhir keterlambatan'), {
    target: { value: '2026-10-13T23:59' },
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
  fireEvent.change(screen.getByLabelText('Output maksimum (byte)'), { target: { value: '65536' } })
  fireEvent.change(screen.getByLabelText('Ukuran tmpfs (MB)'), { target: { value: '64' } })
}

beforeEach(() => {
  localStorage.clear()
  window.history.pushState({}, '', '/lecturer/assignments')
  vi.stubEnv('VITE_USE_MOCK', 'true')
  createLecturerAssignment.mockResolvedValue({
    id: 'assignment-1',
    courseId: 'course-1',
    title: 'Latihan Algoritma',
    description: 'Latihan dasar',
    deadlineAt: '2026-10-12T16:59:00.000Z',
    lateUntilAt: '2026-10-13T16:59:00.000Z',
  })
  createTestCase.mockResolvedValue({ id: 'testcase-1', assignmentId: 'assignment-1' })
  getTestCases.mockResolvedValue([])
  updateResourceLimit.mockResolvedValue({})
  updateLecturerAssignment.mockResolvedValue({
    id: 'assignment-1',
    courseId: 'course-1',
    title: 'Latihan Algoritma',
    description: 'Latihan dasar',
    deadlineAt: '2026-10-12T16:59:00.000Z',
    lateUntilAt: '2026-10-13T16:59:00.000Z',
  })
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
    await fillAssignmentForm()
    fireEvent.click(screen.getByRole('button', { name: 'Buat assignment' }))

    expect(await screen.findByRole('heading', { name: 'Tambah test case' })).toBeInTheDocument()
    expect(createLecturerAssignment).toHaveBeenCalledWith('course-1', {
      title: 'Latihan Algoritma',
      description: 'Latihan dasar',
      deadline_at: new Date('2026-10-12T23:59').toISOString(),
      late_until_at: new Date('2026-10-13T23:59').toISOString(),
    })
    expect(updateResourceLimit).toHaveBeenCalledWith('assignment-1', {
        cpu_time_ms: 1000,
        memory_mb: 128,
        max_processes: 4,
        timeout_s: 5,
        max_output_bytes: 65536,
        tmpfs_size_mb: 64,
    })
  })

  it('rejects non-positive resource limits', async () => {
    setUser()
    render(<App />)
    await fillAssignmentForm()
    fireEvent.change(screen.getByLabelText('Batas memori (MB)'), {
      target: { value: '0' },
    })
    fireEvent.submit(screen.getByRole('heading', { name: 'Buat assignment' }).closest('form'))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Semua batas sumber daya harus berupa bilangan bulat positif.',
    )
    expect(createLecturerAssignment).not.toHaveBeenCalled()
  })

  it('rejects decimal resource limits', async () => {
    setUser()
    render(<App />)
    await fillAssignmentForm()
    fireEvent.change(screen.getByLabelText('Batas waktu CPU (ms)'), {
      target: { value: '1000.5' },
    })
    fireEvent.submit(screen.getByRole('heading', { name: 'Buat assignment' }).closest('form'))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Semua batas sumber daya harus berupa bilangan bulat positif.',
    )
    expect(createLecturerAssignment).not.toHaveBeenCalled()
  })

  it('requires late submission cutoff to be at or after the deadline', async () => {
    setUser()
    render(<App />)
    await fillAssignmentForm()
    fireEvent.change(screen.getByLabelText('Batas akhir keterlambatan'), {
      target: { value: '2026-10-11T23:59' },
    })
    fireEvent.submit(screen.getByRole('heading', { name: 'Buat assignment' }).closest('form'))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Batas akhir keterlambatan harus sama dengan atau setelah deadline.',
    )
    expect(createLecturerAssignment).not.toHaveBeenCalled()
  })

  it('retries a failed resource-limit save without creating another assignment', async () => {
    setUser()
    updateResourceLimit
      .mockRejectedValueOnce(new Error('Resource limit gagal disimpan.'))
      .mockResolvedValueOnce({})
    render(<App />)
    await fillAssignmentForm()
    fireEvent.click(screen.getByRole('button', { name: 'Buat assignment' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Resource limit gagal disimpan.')
    expect(screen.getByRole('heading', { name: 'Ubah assignment' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Simpan perubahan' }))

    expect(await screen.findByText('Assignment berhasil diperbarui.')).toBeInTheDocument()
    expect(createLecturerAssignment).toHaveBeenCalledTimes(1)
    expect(updateLecturerAssignment).toHaveBeenCalledTimes(1)
    expect(updateResourceLimit).toHaveBeenCalledTimes(2)
  })

  it('rejects tmpfs limits that exceed the memory constraint', async () => {
    setUser()
    render(<App />)
    await fillAssignmentForm()
    fireEvent.change(screen.getByLabelText('Batas memori (MB)'), {
      target: { value: '64' },
    })
    fireEvent.submit(screen.getByRole('heading', { name: 'Buat assignment' }).closest('form'))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Ukuran tmpfs ditambah 16 MB tidak boleh melebihi batas memori.',
    )
    expect(createLecturerAssignment).not.toHaveBeenCalled()
  })

  it('rejects an output limit that does not fit twice in tmpfs', async () => {
    setUser()
    render(<App />)
    await fillAssignmentForm()
    fireEvent.change(screen.getByLabelText('Batas memori (MB)'), {
      target: { value: '32' },
    })
    fireEvent.change(screen.getByLabelText('Ukuran tmpfs (MB)'), {
      target: { value: '1' },
    })
    fireEvent.change(screen.getByLabelText('Output maksimum (byte)'), {
      target: { value: '524288' },
    })
    fireEvent.submit(screen.getByRole('heading', { name: 'Buat assignment' }).closest('form'))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Dua kali batas output harus lebih kecil dari ukuran tmpfs dalam byte.',
    )
    expect(createLecturerAssignment).not.toHaveBeenCalled()
  })

  it('adds a test case with the entity field names', async () => {
    setUser('admin')
    render(<App />)
    await fillAssignmentForm()
    fireEvent.click(screen.getByRole('button', { name: 'Buat assignment' }))
    await screen.findByRole('heading', { name: 'Tambah test case' })

    fireEvent.change(screen.getByLabelText('Input'), { target: { value: '2 3' } })
    fireEvent.change(screen.getByLabelText('Keluaran yang diharapkan'), {
      target: { value: '5' },
    })
    fireEvent.change(screen.getByLabelText('Bobot'), { target: { value: '10' } })
    fireEvent.change(screen.getByLabelText('ID eksternal'), { target: { value: 'tc_01' } })
    fireEvent.change(screen.getByLabelText('Mode pencocokan output'), {
      target: { value: 'WHITESPACE_INSENSITIVE' },
    })
    fireEvent.click(screen.getByLabelText('Test case tersembunyi'))
    fireEvent.click(screen.getByRole('button', { name: 'Tambah test case' }))

    expect(await screen.findByText('Test case berhasil ditambahkan.')).toHaveAttribute(
      'role',
      'status',
    )
    expect(createTestCase).toHaveBeenCalledWith('assignment-1', {
      external_id: 'tc_01',
      input: '2 3',
      expected_output: '5',
      weight: 10,
      is_hidden: true,
      match_mode: 'WHITESPACE_INSENSITIVE',
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
