import { apiRequest } from './client.js'

function isIdentifier(value) {
  return typeof value === 'string' || typeof value === 'number'
}

const matchModes = new Set(['EXACT', 'WHITESPACE_INSENSITIVE'])

function readTestCase(payload) {
  if (
    !payload ||
    typeof payload !== 'object' ||
    !isIdentifier(payload.id) ||
    !isIdentifier(payload.assignment_id) ||
    typeof payload.external_id !== 'string' ||
    typeof payload.input !== 'string' ||
    typeof payload.expected_output !== 'string' ||
    !Number.isFinite(payload.weight) ||
    typeof payload.is_hidden !== 'boolean' ||
    !matchModes.has(payload.match_mode)
  ) {
    throw new Error('Respons test case tidak sesuai format yang diharapkan.')
  }

  return {
    id: String(payload.id),
    assignmentId: String(payload.assignment_id),
    externalId: payload.external_id,
    input: payload.input,
    expectedOutput: payload.expected_output,
    weight: payload.weight,
    hidden: payload.is_hidden,
    matchMode: payload.match_mode,
  }
}

export async function getTestCases(assignmentId) {
  if (import.meta.env.VITE_USE_MOCK === 'true') {
    return []
  }

  const response = await apiRequest(
    `/assignments/${encodeURIComponent(assignmentId)}/testcases?page=1&page_size=50`,
  )
  if (!response.ok) {
    throw new Error(`Gagal memuat test case (${response.status}).`)
  }

  const payload = await response.json()
  const testCases = Array.isArray(payload) ? payload : payload.items
  if (!Array.isArray(testCases)) {
    throw new Error('Respons daftar test case tidak sesuai format yang diharapkan.')
  }
  return testCases.map(readTestCase)
}

export async function createTestCase(assignmentId, testCase) {
  let payload
  if (import.meta.env.VITE_USE_MOCK === 'true') {
    payload = { id: 'testcase-created-1', assignment_id: assignmentId, ...testCase }
  } else {
    const response = await apiRequest(
      `/assignments/${encodeURIComponent(assignmentId)}/testcases`,
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(testCase),
      },
    )
    if (!response.ok) {
      throw new Error(`Gagal menambahkan test case (${response.status}).`)
    }
    payload = await response.json()
  }

  return readTestCase(payload)
}

export async function updateTestCase(testCaseId, testCase) {
  let payload
  if (import.meta.env.VITE_USE_MOCK === 'true') {
    payload = { id: testCaseId, assignment_id: 'assignment-created-1', ...testCase }
  } else {
    const response = await apiRequest(`/testcases/${encodeURIComponent(testCaseId)}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(testCase),
    })
    if (!response.ok) {
      throw new Error(`Gagal memperbarui test case (${response.status}).`)
    }
    payload = await response.json()
  }
  return readTestCase(payload)
}

export async function deleteTestCase(testCaseId) {
  if (import.meta.env.VITE_USE_MOCK === 'true') {
    return
  }
  const response = await apiRequest(`/testcases/${encodeURIComponent(testCaseId)}`, {
    method: 'DELETE',
  })
  if (!response.ok) {
    throw new Error(`Gagal menghapus test case (${response.status}).`)
  }
}
