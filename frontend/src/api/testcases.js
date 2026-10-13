import { apiRequest } from './client.js'

function isIdentifier(value) {
  return typeof value === 'string' || typeof value === 'number'
}

function readCreatedTestCase(payload, assignmentId) {
  if (
    !payload ||
    typeof payload !== 'object' ||
    !isIdentifier(payload.id) ||
    !isIdentifier(payload.assignment_id) ||
    String(payload.assignment_id) !== String(assignmentId)
  ) {
    throw new Error('Respons test case tidak sesuai format yang diharapkan.')
  }

  return { id: String(payload.id), assignmentId: String(payload.assignment_id) }
}

export async function createTestCase(assignmentId, testCase) {
  const normalizedId = encodeURIComponent(assignmentId)
  let payload

  if (import.meta.env.VITE_USE_MOCK === 'true') {
    payload = {
      id: 'testcase-created-1',
      assignment_id: assignmentId,
      ...testCase,
    }
  } else {
    const response = await apiRequest(`/assignments/${normalizedId}/testcases`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(testCase),
    })
    if (!response.ok) {
      throw new Error(`Gagal menambahkan test case (${response.status}).`)
    }
    payload = await response.json()
  }

  return readCreatedTestCase(payload, assignmentId)
}
