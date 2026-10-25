import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import App from '../App.jsx'
import {
  createEnrollment,
  getCourseEnrollmentsPage,
  getCourses,
  getCoursesPage,
} from '../api/courses.js'
import { getAssignments } from '../api/assignments.js'
import { createLecturerAssignment } from '../api/lecturerAssignments.js'
import { createTestCase } from '../api/testcases.js'
import { downloadRecapCsv, getAssignmentRecap } from '../api/gradeRecap.js'

vi.mock('../api/courses.js', () => ({
  createCourse: vi.fn(),
  createEnrollment: vi.fn(),
  deleteEnrollment: vi.fn(),
  getCourseEnrollmentsPage: vi.fn(),
  getCourses: vi.fn(),
  getCoursesPage: vi.fn(),
}))

vi.mock('../api/assignments.js', () => ({
  getAssignments: vi.fn(),
}))

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
  exportTestCasesYaml: vi.fn(),
  getTestCases: vi.fn(),
  importTestCasesYaml: vi.fn(),
  updateTestCase: vi.fn(),
}))

vi.mock('../api/gradeRecap.js', () => ({
  downloadRecapCsv: vi.fn(),
  getAssignmentRecap: vi.fn(),
  regradeAssignment: vi.fn(),
  regradeSubmission: vi.fn(),
}))

function setLecturer() {
  localStorage.setItem('token', 'lecturer-token')
  localStorage.setItem('refresh_token', 'lecturer-refresh-token')
  localStorage.setItem(
    'user',
    JSON.stringify({
      id: 'lecturer-1',
      nama: 'Dosen UAT',
      email: 'lecturer@example.test',
      role: 'dosen',
    }),
  )
}

async function fillAssignmentForm() {
  fireEvent.change(await screen.findByLabelText('Judul'), {
    target: { value: 'Latihan UAT' },
  })
  fireEvent.change(screen.getByLabelText('Deskripsi'), {
    target: { value: 'Latihan alur dosen' },
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
    target: { value: '64' },
  })
  fireEvent.change(screen.getByLabelText('Jumlah proses maksimum'), {
    target: { value: '4' },
  })
  fireEvent.change(screen.getByLabelText('Waktu maksimum (detik)'), {
    target: { value: '5' },
  })
  fireEvent.change(screen.getByLabelText('Output maksimum (byte)'), {
    target: { value: '1048576' },
  })
  fireEvent.change(screen.getByLabelText('Ukuran tmpfs (MB)'), {
    target: { value: '32' },
  })
}

beforeEach(() => {
  localStorage.clear()
  window.history.pushState({}, '', '/courses')
  vi.stubEnv('VITE_USE_MOCK', 'false')
  getCoursesPage.mockResolvedValue({
    items: [{ id: 'course-1', code: 'CS101', name: 'Algoritma', term: '2026' }],
    page: 1,
    page_size: 10,
    has_next: false,
  })
  getCourseEnrollmentsPage.mockResolvedValue({
    items: [],
    page: 1,
    page_size: 10,
    has_next: false,
  })
  getCourses.mockResolvedValue([{ id: 'course-1', code: 'CS101', name: 'Algoritma', term: '2026' }])
  createEnrollment.mockResolvedValue({})
  getAssignments.mockResolvedValue([])
  createLecturerAssignment.mockResolvedValue({
    id: 'assignment-1',
    courseId: 'course-1',
    title: 'Latihan UAT',
    description: 'Latihan alur dosen',
    deadlineAt: '2026-10-12T16:59:00.000Z',
    lateUntilAt: '2026-10-13T16:59:00.000Z',
  })
  createTestCase.mockResolvedValue({
    id: 'testcase-1',
    assignmentId: 'assignment-1',
    externalId: 'tc_01',
    input: '1',
    expectedOutput: '2',
    weight: 1,
    hidden: false,
    matchMode: 'EXACT',
  })
  getAssignmentRecap.mockResolvedValue({
    assignmentId: 'assignment-1',
    assignmentTitle: 'Latihan UAT',
    page: 1,
    pageSize: 50,
    hasNextPage: false,
    items: [],
  })
  downloadRecapCsv.mockResolvedValue('rekap-assignment-1.csv')
})

afterEach(() => {
  cleanup()
  localStorage.clear()
  vi.unstubAllEnvs()
  vi.clearAllMocks()
})

describe('lecturer workflow navigation', () => {
  it('connects Course enrollment, assignment, test cases, recap and CSV export', async () => {
    setLecturer()
    render(<App />)

    fireEvent.change(await screen.findByLabelText('Email mahasiswa'), {
      target: { value: 'student@example.test' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Tambah enrollment' }))
    expect(await screen.findByRole('status')).toHaveTextContent(
      'Mahasiswa berhasil ditambahkan ke Course.',
    )
    expect(createEnrollment).toHaveBeenCalledWith('course-1', {
      email: 'student@example.test',
    })

    fireEvent.click(screen.getByRole('link', { name: '2. Assignment & test case' }))
    expect(await screen.findByLabelText('Course')).toHaveValue('course-1')
    expect(window.location.search).toBe('?courseId=course-1')
    await fillAssignmentForm()
    fireEvent.click(screen.getByRole('button', { name: 'Buat assignment' }))
    expect(await screen.findByRole('heading', { name: 'Test case — Latihan UAT' })).toBeInTheDocument()

    createTestCase.mockImplementation(async (_assignmentId, payload) => ({
      id: `testcase-${payload.external_id}`,
      assignmentId: 'assignment-1',
      externalId: payload.external_id,
      input: payload.input,
      expectedOutput: payload.expected_output,
      weight: payload.weight,
      hidden: false,
      matchMode: 'EXACT',
    }))
    for (let index = 1; index <= 5; index += 1) {
      fireEvent.change(screen.getByLabelText('ID eksternal (opsional)'), {
        target: { value: `tc_0${index}` },
      })
      fireEvent.change(screen.getByLabelText('Input'), {
        target: { value: `${index}\n` },
      })
      fireEvent.change(screen.getByLabelText('Keluaran yang diharapkan'), {
        target: { value: `${index * 2}\n` },
      })
      fireEvent.change(screen.getByLabelText('Bobot'), { target: { value: '1' } })
      fireEvent.click(screen.getByRole('button', { name: 'Tambah test case' }))
      await waitFor(() => expect(createTestCase).toHaveBeenCalledTimes(index))
      await waitFor(() => expect(screen.getByLabelText('Input')).toHaveValue(''))
    }
    expect(screen.getByRole('heading', { name: 'tc_05' })).toBeInTheDocument()
    expect(createTestCase).toHaveBeenCalledWith('assignment-1', expect.objectContaining({
      input: '1\n',
      expected_output: '2\n',
      weight: 1,
    }))

    const recapLink = screen.getByRole('link', { name: 'Rekap nilai & CSV' })
    expect(recapLink).toHaveAttribute(
      'href',
      '/lecturer/assignments/assignment-1/recap?courseId=course-1',
    )
    fireEvent.click(recapLink)
    expect(await screen.findByText('Latihan UAT')).toBeInTheDocument()
    expect(getAssignmentRecap).toHaveBeenCalledWith('assignment-1', {
      page: 1,
      pageSize: 50,
    })

    fireEvent.click(screen.getByRole('button', { name: 'Unduh CSV' }))
    expect(await screen.findByRole('status')).toHaveTextContent('rekap-assignment-1.csv')
    expect(downloadRecapCsv).toHaveBeenCalledWith('assignment-1')

    fireEvent.click(screen.getByRole('link', { name: '1. Course & enrollment' }))
    expect(window.location.search).toBe('?courseId=course-1')
    expect(await screen.findByRole('button', { name: /CS101 — Algoritma/ })).toHaveAttribute(
      'aria-pressed',
      'true',
    )
    fireEvent.click(screen.getByRole('link', { name: '2. Assignment & test case' }))
    expect(await screen.findByLabelText('Course')).toHaveValue('course-1')
  }, 15_000)

  it('does not reload Courses when the selected Course query changes', async () => {
    setLecturer()
    getCourses.mockResolvedValue([
      { id: 'course-1', code: 'CS101', name: 'Algoritma', term: '2026' },
      { id: 'course-2', code: 'CS102', name: 'Struktur Data', term: '2026' },
    ])
    window.history.pushState({}, '', '/lecturer/assignments?courseId=course-2')
    render(<App />)

    const courseSelect = await screen.findByLabelText('Course')
    expect(courseSelect).toHaveValue('course-2')
    fireEvent.change(courseSelect, { target: { value: 'course-1' } })

    await waitFor(() => expect(window.location.search).toBe('?courseId=course-1'))
    expect(getCourses).toHaveBeenCalledTimes(1)
  })

  it('creates five test cases before opening the recap flow', async () => {
    setLecturer()
    render(<App />)

    fireEvent.click(await screen.findByRole('link', { name: '2. Assignment & test case' }))
    await fillAssignmentForm()
    fireEvent.click(screen.getByRole('button', { name: 'Buat assignment' }))
    await screen.findByRole('heading', { name: 'Test case — Latihan UAT' })

    createTestCase.mockImplementation(async (_assignmentId, payload) => ({
      id: `testcase-${payload.external_id}`,
      assignmentId: 'assignment-1',
      externalId: payload.external_id,
      input: payload.input,
      expectedOutput: payload.expected_output,
      weight: payload.weight,
      hidden: false,
      matchMode: 'EXACT',
    }))
    for (let index = 1; index <= 5; index += 1) {
      fireEvent.change(screen.getByLabelText('ID eksternal (opsional)'), {
        target: { value: `tc_0${index}` },
      })
      fireEvent.change(screen.getByLabelText('Input'), { target: { value: `${index}\n` } })
      fireEvent.change(screen.getByLabelText('Keluaran yang diharapkan'), {
        target: { value: `${index * 2}\n` },
      })
      fireEvent.change(screen.getByLabelText('Bobot'), { target: { value: '1' } })
      fireEvent.click(screen.getByRole('button', { name: 'Tambah test case' }))
      await waitFor(() => expect(createTestCase).toHaveBeenCalledTimes(index))
      await waitFor(() => expect(screen.getByLabelText('Input')).toHaveValue(''))
    }

    expect(screen.getByRole('heading', { name: 'tc_05' })).toBeInTheDocument()
    expect(createTestCase).toHaveBeenCalledTimes(5)
  }, 15_000)
})
