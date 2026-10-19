import { apiRequest, throwApiError, throwMockApiError } from './client.js'

function isIdentifier(value) {
  return typeof value === 'string' || typeof value === 'number'
}

const matchModes = new Set(['EXACT', 'WHITESPACE_INSENSITIVE'])
const mockTestCaseDocument = `schema_version: 1
assignment: Demo assignment
resource_limit:
  cpu_time_ms: 1000
  memory_mb: 64
  tmpfs_size_mb: 32
  max_processes: 4
  timeout_s: 5
  max_output_bytes: 1048576
test_cases: []
`

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
    await throwApiError(response, `Gagal memuat test case (${response.status}).`)
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
      await throwApiError(response, `Gagal menambahkan test case (${response.status}).`)
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
      await throwApiError(response, `Gagal memperbarui test case (${response.status}).`)
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
    await throwApiError(response, `Gagal menghapus test case (${response.status}).`)
  }
}

export async function exportTestCasesYaml(assignmentId) {
  if (import.meta.env.VITE_USE_MOCK === 'true') {
    return {
      blob: new Blob([mockTestCaseDocument], { type: 'application/yaml;charset=utf-8' }),
      filename: `testcases-assignment-${assignmentId}.yaml`,
    }
  }

  const response = await apiRequest(
    `/assignments/${encodeURIComponent(assignmentId)}/testcases/export`,
  )
  if (!response.ok) {
    await throwApiError(response, `Gagal mengekspor YAML (${response.status}).`)
  }

  const disposition = response.headers.get('Content-Disposition') ?? ''
  const filename = /filename="([^"]+)"/i.exec(disposition)?.[1]
  return {
    blob: await response.blob(),
    filename: filename || `testcases-assignment-${assignmentId}.yaml`,
  }
}

export async function importTestCasesYaml(assignmentId, document) {
  if (typeof document !== 'string' || !document.trim()) {
    await throwMockApiError('invalid_yaml', 'Berkas YAML tidak boleh kosong.')
  }

  if (import.meta.env.VITE_USE_MOCK === 'true') {
    if (!/^\s*schema_version:\s*1\s*$/m.test(document) || !/^\s*test_cases\s*:/m.test(document)) {
      await throwMockApiError(
        'invalid_yaml',
        'YAML harus memuat schema_version: 1 dan test_cases.',
      )
    }
    const created = document.match(/^\s*-\s+external_id\s*:/gm)?.length ?? 0
    return { created, updated: 0 }
  }

  const response = await apiRequest(
    `/assignments/${encodeURIComponent(assignmentId)}/testcases/import`,
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/yaml' },
      body: document,
    },
  )
  if (!response.ok) {
    await throwApiError(response, `Gagal mengimpor YAML (${response.status}).`)
  }

  const payload = await response.json()
  if (
    !payload ||
    !Number.isInteger(payload.created) ||
    payload.created < 0 ||
    !Number.isInteger(payload.updated) ||
    payload.updated < 0
  ) {
    throw new Error('Respons impor YAML tidak sesuai format yang diharapkan.')
  }
  return { created: payload.created, updated: payload.updated }
}
