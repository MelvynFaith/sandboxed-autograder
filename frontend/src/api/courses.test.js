import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  createCourse,
  createEnrollment,
  deleteEnrollment,
  getCourseAssignmentsPage,
  getCourseEnrollmentsPage,
  getCourses,
  getCoursesPage,
} from './courses.js'

beforeEach(() => {
  localStorage.clear()
  vi.stubEnv('VITE_USE_MOCK', 'false')
})

afterEach(() => {
  localStorage.clear()
  vi.unstubAllEnvs()
  vi.unstubAllGlobals()
})

describe('course API adapters', () => {
  it('preserves pagination metadata when listing courses', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        items: [{ id: 12, code: 'CS101', name: 'Algoritma', term: '2026' }],
        page: 2,
        page_size: 10,
        has_next: true,
      }),
    })
    vi.stubGlobal('fetch', fetchMock)

    await expect(getCoursesPage({ page: 2, pageSize: 10 })).resolves.toEqual({
      items: [{ id: '12', code: 'CS101', name: 'Algoritma', term: '2026' }],
      page: 2,
      page_size: 10,
      has_next: true,
    })
    expect(fetchMock).toHaveBeenCalledWith('/api/v1/courses?page=2&page_size=10', expect.any(Object))
  })

  it('loads all Course pages for the lecturer assignment selector', async () => {
    const firstPage = Array.from({ length: 50 }, (_, index) => ({
      id: index + 1,
      code: `CS${String(index + 1).padStart(3, '0')}`,
      name: `Course ${index + 1}`,
      term: '2026',
    }))
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          items: firstPage,
          page: 1,
          page_size: 50,
          has_next: true,
        }),
      })
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          items: [{ id: 51, code: 'CS051', name: 'Course 51', term: '2026' }],
          page: 2,
          page_size: 50,
          has_next: false,
        }),
      })
    vi.stubGlobal('fetch', fetchMock)

    const courses = await getCourses()

    expect(courses).toHaveLength(51)
    expect(courses.at(-1)).toMatchObject({ id: '51', code: 'CS051' })
    expect(fetchMock.mock.calls.map(([url]) => url)).toEqual([
      '/api/v1/courses?page=1&page_size=50',
      '/api/v1/courses?page=2&page_size=50',
    ])
  })

  it('limits mock student Course listings to the current user enrollments', async () => {
    vi.stubEnv('VITE_USE_MOCK', 'true')
    localStorage.setItem(
      'user',
      JSON.stringify({ id: 'mock-student', role: 'mahasiswa' }),
    )

    await expect(getCoursesPage()).resolves.toMatchObject({
      items: [{ id: 'course-1', code: 'CS101' }],
      has_next: false,
    })
  })

  it('uses the agreed request bodies for courses and enrollments', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({ id: 12, code: 'CS101', name: 'Algoritma', term: '2026' }),
      })
      .mockResolvedValueOnce({ ok: true, status: 204 })
      .mockResolvedValueOnce({ ok: true, status: 204 })
    vi.stubGlobal('fetch', fetchMock)

    await createCourse({ name: 'Algoritma', code: 'CS101', term: '2026' }, 'dosen')
    await createEnrollment('12', { email: 'student@example.test' })
    await deleteEnrollment('12', '9')

    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/courses')
    expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({
      name: 'Algoritma',
      code: 'CS101',
      term: '2026',
    })
    expect(fetchMock.mock.calls[1][0]).toBe('/api/v1/courses/12/enrollments')
    expect(JSON.parse(fetchMock.mock.calls[1][1].body)).toEqual({ email: 'student@example.test' })
    expect(fetchMock.mock.calls[2][0]).toBe('/api/v1/courses/12/enrollments/9')
    expect(fetchMock.mock.calls[2][1].method).toBe('DELETE')
  })

  it('includes lecturer_email only when an administrator creates a course', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ id: 12, code: 'CS101', name: 'Algoritma', term: '2026' }),
    })
    vi.stubGlobal('fetch', fetchMock)

    await createCourse({
      name: 'Algoritma',
      code: 'CS101',
      term: '2026',
      lecturerEmail: 'dosen@example.test',
    }, 'admin')

    expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({
      name: 'Algoritma',
      code: 'CS101',
      term: '2026',
      lecturer_email: 'dosen@example.test',
    })
  })

  it('requests the selected enrollment and course assignment pages', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          items: [{ id: 3, user_id: 9, name: 'Siswa', email: 'siswa@example.test' }],
          page: 2,
          page_size: 10,
          has_next: false,
        }),
      })
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          items: [{
            id: 7,
            course_id: 12,
            title: 'Tugas',
            description: 'Deskripsi',
            deadline_at: '2026-10-12T16:59:00Z',
            late_until_at: '2026-10-13T16:59:00Z',
          }],
          page: 3,
          page_size: 10,
          has_next: true,
        }),
      })
    vi.stubGlobal('fetch', fetchMock)

    const enrollments = await getCourseEnrollmentsPage('12', { page: 2, pageSize: 10 })
    const assignments = await getCourseAssignmentsPage('12', { page: 3, pageSize: 10 })

    expect(enrollments.items[0]).toMatchObject({
      userId: '9',
      name: 'Siswa',
      email: 'siswa@example.test',
    })
    expect(enrollments.has_next).toBe(false)
    expect(assignments.items[0]).toMatchObject({ courseId: '12', title: 'Tugas' })
    expect(assignments).toMatchObject({ page: 3, page_size: 10, has_next: true })
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/courses/12/enrollments?page=2&page_size=10')
    expect(fetchMock.mock.calls[1][0]).toBe('/api/v1/courses/12/assignments?page=3&page_size=10')
  })

  it('displays API error messages for create course and enrollment failures', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce({
        ok: false,
        status: 409,
        json: async () => ({ error: { code: 'course_code_exists', message: 'Kode sudah digunakan.' } }),
      })
      .mockResolvedValueOnce({
        ok: false,
        status: 404,
        json: async () => ({ error: { code: 'not_found', message: 'Email tidak ditemukan.' } }),
      })
    vi.stubGlobal('fetch', fetchMock)

    await expect(createCourse({ name: 'Algo', code: 'CS101', term: '2026' }, 'dosen')).rejects.toThrow(
      'Kode sudah digunakan.',
    )
    await expect(createEnrollment('1', { email: 'missing@example.test' })).rejects.toThrow(
      'Email tidak ditemukan.',
    )
  })

  it('matches backend administrator lecturer validation in mock mode', async () => {
    vi.stubEnv('VITE_USE_MOCK', 'true')

    await expect(createCourse({ name: 'Algo', code: 'CS201', term: '2026' }, 'admin'))
      .rejects.toThrow('Admin wajib menentukan email dosen pengampu.')
    await expect(createCourse({
      name: 'Algo',
      code: 'CS202',
      term: '2026',
      lecturerEmail: 'missing@example.test',
    }, 'admin')).rejects.toThrow('Dosen dengan email tersebut tidak ditemukan.')
    await expect(createCourse({
      name: 'Algo',
      code: 'CS203',
      term: '2026',
      lecturerEmail: 'mahasiswa@example.test',
    }, 'admin')).rejects.toThrow('Pengguna yang dipilih bukan dosen dengan peran LECTURER.')
  })
})
