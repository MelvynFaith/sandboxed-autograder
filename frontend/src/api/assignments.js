import { apiRequest, throwApiError } from './client.js'

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

function readAssignmentsResponse(payload) {
  const assignments = Array.isArray(payload) ? payload : payload?.items
  if (
    !Array.isArray(assignments) ||
    assignments.some(
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

  return assignments.map((assignment) => ({
    id: String(assignment.id),
    courseId: String(assignment.course_id),
    title: assignment.title,
    description: assignment.description,
    deadlineAt: assignment.deadline_at,
    lateUntilAt: assignment.late_until_at,
  }))
}

export async function getAssignments({ courseId } = {}) {
  if (import.meta.env.VITE_USE_MOCK === 'true') {
    const assignments = readAssignmentsResponse(mockAssignments)
    return courseId
      ? assignments.filter((assignment) => assignment.courseId === String(courseId))
      : assignments
  }

  const query = '?page=1&page_size=50'
  const path = courseId
    ? `/courses/${encodeURIComponent(courseId)}/assignments${query}`
    : `/assignments${query}`
  const response = await apiRequest(path)
  if (!response.ok) {
    await throwApiError(response, `Gagal memuat assignment (${response.status}).`)
  }

  return readAssignmentsResponse(await response.json())
}
