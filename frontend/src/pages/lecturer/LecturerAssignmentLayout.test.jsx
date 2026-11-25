import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import App from '../../App.jsx'
import { getAssignment, getAssignments } from '../../api/assignments.js'
import { getCourses } from '../../api/courses.js'
import { getResourceLimit } from '../../api/lecturerAssignments.js'
import { getTestCases } from '../../api/testcases.js'

vi.mock('../../api/assignments.js', () => ({ getAssignment: vi.fn(), getAssignments: vi.fn() }))
vi.mock('../../api/courses.js', () => ({ getCourses: vi.fn() }))
vi.mock('../../api/lecturerAssignments.js', () => ({ getResourceLimit: vi.fn() }))
vi.mock('../../api/testcases.js', () => ({ getTestCases: vi.fn() }))

const assignment = {
  id: '7',
  courseId: '3',
  title: 'Deret Fibonacci',
  description: 'Hitung deret.',
  deadlineAt: '2026-10-12T16:59:00.000Z',
  lateUntilAt: '2026-10-13T16:59:00.000Z',
}

function setUser(role) {
  localStorage.setItem('token', 'token')
  localStorage.setItem('refresh_token', 'refresh')
  localStorage.setItem(
    'user',
    JSON.stringify({ id: 'u1', nama: 'Dosen', email: 'dosen@example.test', role }),
  )
}

beforeEach(() => {
  localStorage.clear()
  getAssignment.mockResolvedValue(assignment)
  getAssignments.mockResolvedValue([assignment])
  getCourses.mockResolvedValue([{ id: '3', code: 'CS101', name: 'Algoritma', term: '2026' }])
  getResourceLimit.mockResolvedValue({
    cpu_time_ms: 1000,
    memory_mb: 64,
    max_processes: 4,
    timeout_s: 5,
    max_output_bytes: 1048576,
    tmpfs_size_mb: 32,
  })
  getTestCases.mockResolvedValue([])
})

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

describe('lecturer assignment layout', () => {
  it('shows the title, the deadlines and three tabs that keep the course in the address', async () => {
    setUser('dosen')
    window.history.pushState({}, '', '/lecturer/assignments/7?courseId=3')
    render(<App />)

    expect(await screen.findByRole('heading', { level: 1, name: 'Deret Fibonacci' })).toBeInTheDocument()
    const tabs = within(screen.getByRole('navigation', { name: 'Bagian assignment' }))
    expect(tabs.getByRole('link', { name: 'Soal dan pengaturan' })).toHaveAttribute('aria-current', 'page')
    expect(tabs.getByRole('link', { name: 'Test case' })).toHaveAttribute(
      'href',
      '/lecturer/assignments/7/testcases?courseId=3',
    )
    expect(tabs.getByRole('link', { name: 'Rekap nilai' })).toHaveAttribute(
      'href',
      '/lecturer/assignments/7/recap?courseId=3',
    )
    expect(
      within(screen.getByRole('navigation', { name: 'Breadcrumb' })).getByRole('link', {
        name: 'Kelola Assignment',
      }),
    ).toHaveAttribute('href', '/lecturer/assignments?courseId=3')

    fireEvent.click(tabs.getByRole('link', { name: 'Test case' }))
    expect(await screen.findByRole('heading', { name: 'Test case — Deret Fibonacci' })).toBeInTheDocument()
    expect(tabs.getByRole('link', { name: 'Test case' })).toHaveAttribute('aria-current', 'page')
  })

  it('says so when the assignment cannot be loaded', async () => {
    setUser('dosen')
    getAssignment.mockRejectedValue(new Error('Assignment tidak ditemukan.'))
    window.history.pushState({}, '', '/lecturer/assignments/99')
    render(<App />)

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Gagal memuat assignment: Assignment tidak ditemukan.',
    )
  })

  it('keeps students out of the assignment pages', async () => {
    setUser('mahasiswa')
    window.history.pushState({}, '', '/lecturer/assignments/7')
    render(<App />)

    expect(await screen.findByRole('navigation', { name: 'Navigasi utama' })).toBeInTheDocument()
    expect(getAssignment).not.toHaveBeenCalled()
    expect(window.location.pathname).toBe('/assignments')
  })

  it('lists the assignments of a course with a link to each page and to the new-assignment form', async () => {
    setUser('dosen')
    window.history.pushState({}, '', '/lecturer/assignments?courseId=3')
    render(<App />)

    const row = await screen.findByRole('row', { name: /Deret Fibonacci/ })
    expect(within(row).getByRole('link', { name: 'Kelola Deret Fibonacci' })).toHaveAttribute(
      'href',
      '/lecturer/assignments/7?courseId=3',
    )
    expect(within(row).getByRole('link', { name: 'Rekap nilai Deret Fibonacci' })).toHaveAttribute(
      'href',
      '/lecturer/assignments/7/recap?courseId=3',
    )
    expect(screen.getByRole('link', { name: 'Buat assignment' })).toHaveAttribute(
      'href',
      '/lecturer/assignments/new?courseId=3',
    )
  })
})
