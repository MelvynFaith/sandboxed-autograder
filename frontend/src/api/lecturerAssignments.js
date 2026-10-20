import { apiRequest, throwApiError } from './client.js'

const resourceLimitFields = [
  'cpu_time_ms',
  'memory_mb',
  'max_processes',
  'timeout_s',
  'max_output_bytes',
  'tmpfs_size_mb',
]

function isIdentifier(value) {
  return typeof value === 'string' || typeof value === 'number'
}

function readAssignment(payload) {
  if (
    !payload ||
    typeof payload !== 'object' ||
    !isIdentifier(payload.id) ||
    (typeof payload.course_id !== 'string' && typeof payload.course_id !== 'number') ||
    typeof payload.title !== 'string' ||
    typeof payload.description !== 'string' ||
    typeof payload.deadline_at !== 'string' ||
    !Number.isFinite(Date.parse(payload.deadline_at)) ||
    typeof payload.late_until_at !== 'string' ||
    !Number.isFinite(Date.parse(payload.late_until_at))
  ) {
    throw new Error('Respons assignment tidak sesuai format yang diharapkan.')
  }

  return {
    id: String(payload.id),
    courseId: String(payload.course_id),
    title: payload.title,
    description: payload.description,
    deadlineAt: payload.deadline_at,
    lateUntilAt: payload.late_until_at,
  }
}

// An assignment always has exactly one ResourceLimit (SRS 7.1): the API creates both in one
// transaction and rejects a body without `resource_limit`.
export async function createLecturerAssignment(courseId, assignment, resourceLimit) {
  let payload

  if (import.meta.env.VITE_USE_MOCK === 'true') {
    payload = {
      id: 'assignment-created-1',
      course_id: courseId,
      ...assignment,
    }
  } else {
    const response = await apiRequest(
      `/courses/${encodeURIComponent(courseId)}/assignments`,
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ...assignment, resource_limit: resourceLimit }),
      },
    )
    if (!response.ok) {
      await throwApiError(response, `Gagal membuat assignment (${response.status}).`)
    }
    payload = await response.json()
  }

  return readAssignment(payload)
}

// The body holds only the assignment fields; the API rejects unknown fields such as `course_id`.
// Resource limits change through updateResourceLimit.
export async function updateLecturerAssignment(assignmentId, assignment) {
  let payload
  if (import.meta.env.VITE_USE_MOCK === 'true') {
    payload = { id: assignmentId, course_id: 'course-1', ...assignment }
  } else {
    const response = await apiRequest(`/assignments/${encodeURIComponent(assignmentId)}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(assignment),
    })
    if (!response.ok) {
      await throwApiError(response, `Gagal memperbarui assignment (${response.status}).`)
    }
    payload = await response.json()
  }
  return readAssignment(payload)
}

export async function deleteLecturerAssignment(assignmentId) {
  if (import.meta.env.VITE_USE_MOCK === 'true') {
    return
  }
  const response = await apiRequest(`/assignments/${encodeURIComponent(assignmentId)}`, {
    method: 'DELETE',
  })
  if (!response.ok) {
    await throwApiError(response, `Gagal menghapus assignment (${response.status}).`)
  }
}

export async function updateResourceLimit(assignmentId, resourceLimit) {
  if (import.meta.env.VITE_USE_MOCK === 'true') {
    return resourceLimit
  }
  const response = await apiRequest(
    `/assignments/${encodeURIComponent(assignmentId)}/resource-limit`,
    {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(resourceLimit),
    },
  )
  if (!response.ok) {
    await throwApiError(response, `Gagal memperbarui batas sumber daya (${response.status}).`)
  }
  return response.json()
}

export async function getResourceLimit(assignmentId) {
  if (import.meta.env.VITE_USE_MOCK === 'true') {
    return {
      cpu_time_ms: 1000,
      memory_mb: 128,
      max_processes: 4,
      timeout_s: 5,
      max_output_bytes: 65536,
      tmpfs_size_mb: 64,
    }
  }
  const response = await apiRequest(
    `/assignments/${encodeURIComponent(assignmentId)}/resource-limit`,
  )
  if (!response.ok) {
    await throwApiError(response, `Gagal memuat batas sumber daya (${response.status}).`)
  }
  const limits = await response.json()
  if (
    resourceLimitFields.some((name) => !Number.isFinite(limits?.[name]) || limits[name] <= 0)
  ) {
    throw new Error('Respons batas sumber daya tidak sesuai format yang diharapkan.')
  }
  return limits
}
