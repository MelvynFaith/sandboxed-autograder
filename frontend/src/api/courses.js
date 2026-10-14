import { apiRequest } from './client.js'

const mockCourses = [{ id: 'course-1', code: 'CS101', name: 'Pemrograman Dasar', term: '2026' }]

function readCourses(payload) {
  if (
    !Array.isArray(payload) ||
    payload.some(
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

  return payload.map((course) => ({
    id: String(course.id),
    code: course.code,
    name: course.name,
    term: course.term,
  }))
}

export async function getCourses() {
  if (import.meta.env.VITE_USE_MOCK === 'true') {
    return readCourses(mockCourses)
  }

  const response = await apiRequest('/courses?page=1&page_size=50')
  if (!response.ok) {
    throw new Error(`Gagal memuat Course (${response.status}).`)
  }

  const payload = await response.json()
  return readCourses(Array.isArray(payload) ? payload : payload.items)
}
