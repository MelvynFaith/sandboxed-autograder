import { apiRequest, throwApiError } from './client.js'

const MOCK_PAGE_SIZE = 10
const mockCourses = [
  { id: 'course-1', code: 'CS101', name: 'Pemrograman Dasar', term: '2026' },
  { id: 'course-2', code: 'CS102', name: 'Struktur Data', term: '2026' },
]
const mockEnrollments = {
  'course-1': [
    { id: 'enroll-1', user_id: 'user-1', name: 'Mahasiswa Demo', email: 'mahasiswa@example.test' },
  ],
  'course-2': [],
}
const mockAssignments = [
  {
    id: 'assignment-1',
    course_id: 'course-1',
    title: 'Dasar Algoritma',
    description: 'Buat program Python untuk menyelesaikan soal algoritma dasar.',
    deadline_at: '2026-10-12T23:59:00+07:00',
    late_until_at: '2026-10-13T23:59:00+07:00',
  },
  {
    id: 'assignment-2',
    course_id: 'course-1',
    title: 'Struktur Data',
    description: 'Implementasikan struktur data yang diminta menggunakan Python.',
    deadline_at: '2026-10-19T23:59:00+07:00',
    late_until_at: '2026-10-20T23:59:00+07:00',
  },
]

function readPage(payload, readItems, label) {
  if (
    !payload ||
    typeof payload !== 'object' ||
    !Array.isArray(payload.items) ||
    !Number.isInteger(payload.page) ||
    payload.page < 1 ||
    !Number.isInteger(payload.page_size) ||
    payload.page_size < 1 ||
    typeof payload.has_next !== 'boolean'
  ) {
    throw new Error(`Respons daftar ${label} tidak sesuai format yang diharapkan.`)
  }

  return {
    items: readItems(payload.items),
    page: payload.page,
    page_size: payload.page_size,
    has_next: payload.has_next,
  }
}

function readCourses(payload) {
  return readPage(
    payload,
    (items) => {
      if (
        items.some(
          (course) =>
            !course ||
            typeof course !== 'object' ||
            (typeof course.id !== 'string' && typeof course.id !== 'number') ||
            typeof course.code !== 'string' ||
            typeof course.name !== 'string' ||
            typeof course.term !== 'string',
        )
      ) {
        throw new Error('Respons daftar Course tidak sesuai format yang diharapkan.')
      }

      return items.map((course) => ({
        id: String(course.id),
        code: course.code,
        name: course.name,
        term: course.term,
      }))
    },
    'Course',
  )
}

function readEnrollments(payload) {
  return readPage(
    payload,
    (items) => {
      if (
        items.some(
          (entry) =>
            !entry ||
            typeof entry !== 'object' ||
            (typeof entry.user_id !== 'string' && typeof entry.user_id !== 'number') ||
            typeof entry.name !== 'string' ||
            typeof entry.email !== 'string',
        )
      ) {
        throw new Error('Respons daftar enrollment tidak sesuai format yang diharapkan.')
      }

      return items.map((entry) => ({
        id: String(entry.id),
        userId: String(entry.user_id),
        name: entry.name,
        email: entry.email,
      }))
    },
    'enrollment',
  )
}

function readAssignments(payload) {
  return readPage(
    payload,
    (items) => {
      if (
        items.some(
          (assignment) =>
            !assignment ||
            typeof assignment !== 'object' ||
            (typeof assignment.id !== 'string' && typeof assignment.id !== 'number') ||
            (typeof assignment.course_id !== 'string' &&
              typeof assignment.course_id !== 'number') ||
            typeof assignment.title !== 'string' ||
            typeof assignment.description !== 'string' ||
            typeof assignment.deadline_at !== 'string' ||
            !Number.isFinite(Date.parse(assignment.deadline_at)) ||
            typeof assignment.late_until_at !== 'string' ||
            !Number.isFinite(Date.parse(assignment.late_until_at)),
        )
      ) {
        throw new Error('Respons daftar assignment tidak sesuai format yang diharapkan.')
      }

      return items.map((assignment) => ({
        id: String(assignment.id),
        courseId: String(assignment.course_id),
        title: assignment.title,
        description: assignment.description,
        deadlineAt: assignment.deadline_at,
        lateUntilAt: assignment.late_until_at,
      }))
    },
    'assignment',
  )
}

function mockPage(items, page, pageSize) {
  const start = (page - 1) * pageSize
  return {
    items: items.slice(start, start + pageSize),
    page,
    page_size: pageSize,
    has_next: start + pageSize < items.length,
  }
}

function queryFor(page, pageSize) {
  return `page=${page}&page_size=${pageSize}`
}

export async function getCoursesPage({ page = 1, pageSize = MOCK_PAGE_SIZE } = {}) {
  if (import.meta.env.VITE_USE_MOCK === 'true') {
    return readCourses(mockPage(mockCourses, page, pageSize))
  }

  const response = await apiRequest(`/courses?${queryFor(page, pageSize)}`)
  if (!response.ok) {
    await throwApiError(response, `Gagal memuat Course (${response.status}).`)
  }

  return readCourses(await response.json())
}

export async function getCourses() {
  return (await getCoursesPage({ pageSize: 50 })).items
}

export async function createCourse(course) {
  if (import.meta.env.VITE_USE_MOCK === 'true') {
    if (mockCourses.some((existing) => existing.code === course.code)) {
      throw new Error('Kode Course sudah digunakan.')
    }
    const created = {
      id: `course-${mockCourses.length + 1}`,
      code: course.code,
      name: course.name,
      term: course.term,
    }
    mockCourses.push(created)
    mockEnrollments[created.id] = []
    return created
  }

  const response = await apiRequest('/courses', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      name: course.name,
      code: course.code,
      term: course.term,
    }),
  })
  if (!response.ok) {
    await throwApiError(response, `Gagal membuat Course (${response.status}).`)
  }

  return await response.json()
}

export async function getCourseEnrollmentsPage(courseId, { page = 1, pageSize = MOCK_PAGE_SIZE } = {}) {
  if (import.meta.env.VITE_USE_MOCK === 'true') {
    return readEnrollments(mockPage(mockEnrollments[courseId] ?? [], page, pageSize))
  }

  const response = await apiRequest(
    `/courses/${encodeURIComponent(courseId)}/enrollments?${queryFor(page, pageSize)}`,
  )
  if (!response.ok) {
    await throwApiError(response, `Gagal memuat enrollment (${response.status}).`)
  }

  return readEnrollments(await response.json())
}

export async function getCourseEnrollments(courseId) {
  return (await getCourseEnrollmentsPage(courseId)).items
}

export async function createEnrollment(courseId, { email }) {
  if (import.meta.env.VITE_USE_MOCK === 'true') {
    if (email === 'dosen@example.test' || email === 'admin@example.test') {
      throw new Error('Hanya akun mahasiswa yang dapat didaftarkan.')
    }
    if (email !== 'mahasiswa@example.test') {
      throw new Error('Pengguna dengan email tersebut tidak ditemukan.')
    }

    const collection = mockEnrollments[courseId] ?? []
    if (collection.some((entry) => entry.email === email)) {
      throw new Error('Mahasiswa sudah terdaftar pada Course ini.')
    }

    const created = {
      id: `enroll-${collection.length + 1}`,
      user_id: `user-${collection.length + 1}`,
      name: 'Mahasiswa Demo',
      email,
    }
    mockEnrollments[courseId] = [...collection, created]
    return created
  }

  const response = await apiRequest(`/courses/${encodeURIComponent(courseId)}/enrollments`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email }),
  })
  if (!response.ok) {
    await throwApiError(response, `Gagal mendaftarkan mahasiswa (${response.status}).`)
  }

  return true
}

export async function deleteEnrollment(courseId, userId) {
  if (import.meta.env.VITE_USE_MOCK === 'true') {
    const collection = mockEnrollments[courseId] ?? []
    mockEnrollments[courseId] = collection.filter((entry) => String(entry.user_id) !== String(userId))
    return true
  }

  const response = await apiRequest(
    `/courses/${encodeURIComponent(courseId)}/enrollments/${encodeURIComponent(userId)}`,
    { method: 'DELETE' },
  )
  if (!response.ok) {
    await throwApiError(response, `Gagal menghapus enrollment (${response.status}).`)
  }
  return true
}

export async function getCourseAssignmentsPage(
  courseId,
  { page = 1, pageSize = MOCK_PAGE_SIZE } = {},
) {
  if (import.meta.env.VITE_USE_MOCK === 'true') {
    const items = mockAssignments.filter(
      (assignment) => String(assignment.course_id) === String(courseId),
    )
    return readAssignments(mockPage(items, page, pageSize))
  }

  const response = await apiRequest(
    `/courses/${encodeURIComponent(courseId)}/assignments?${queryFor(page, pageSize)}`,
  )
  if (!response.ok) {
    await throwApiError(response, `Gagal memuat assignment (${response.status}).`)
  }

  return readAssignments(await response.json())
}
