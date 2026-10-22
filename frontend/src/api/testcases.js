import { apiRequest, throwApiError, throwMockApiError } from './client.js'

function isIdentifier(value) {
  return typeof value === 'string' || typeof value === 'number'
}

const matchModes = new Set(['EXACT', 'WHITESPACE_INSENSITIVE'])
const MOCK_TEST_CASES_KEY = 'sandboxed_autograder_mock_test_cases'
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

function readMockTestCases() {
  const serialized = localStorage.getItem(MOCK_TEST_CASES_KEY)
  if (!serialized) {
    return []
  }

  let testCases
  try {
    testCases = JSON.parse(serialized)
  } catch (error) {
    if (!(error instanceof SyntaxError)) {
      throw error
    }
    throw new Error('Data test case mock tidak valid.')
  }
  if (!Array.isArray(testCases)) {
    throw new Error('Data test case mock tidak valid.')
  }
  return testCases
}

function writeMockTestCases(testCases) {
  localStorage.setItem(MOCK_TEST_CASES_KEY, JSON.stringify(testCases))
}

function nextMockTestCaseId(testCases) {
  let nextId = testCases.length + 1
  while (testCases.some((testCase) => testCase.id === `testcase-${nextId}`)) {
    nextId += 1
  }
  return `testcase-${nextId}`
}

function generatedExternalId(testCases, assignmentId) {
  for (let nextId = 1; ; nextId += 1) {
    const candidate = `tc_${String(nextId).padStart(2, '0')}`
    if (!testCases.some(
      (testCase) =>
        String(testCase.assignment_id) === String(assignmentId) &&
        testCase.external_id === candidate,
    )) {
      return candidate
    }
  }
}

function readTestCase(payload) {
  if (
    !payload ||
    typeof payload !== 'object' ||
    !isIdentifier(payload.id) ||
    !isIdentifier(payload.assignment_id) ||
    typeof payload.external_id !== 'string' ||
    typeof payload.input !== 'string' ||
    typeof payload.expected_output !== 'string' ||
    !Number.isInteger(payload.weight) ||
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
    return readMockTestCases()
      .filter((testCase) => String(testCase.assignment_id) === String(assignmentId))
      .map(readTestCase)
  }

  const results = []
  for (let page = 1; ; page += 1) {
    const response = await apiRequest(
      `/assignments/${encodeURIComponent(assignmentId)}/testcases?page=${page}&page_size=50`,
    )
    if (!response.ok) {
      await throwApiError(response, `Gagal memuat test case (${response.status}).`)
    }

    const payload = await response.json()
    if (Array.isArray(payload)) {
      results.push(...payload.map(readTestCase))
      break
    }
    if (
      !payload ||
      !Array.isArray(payload.items) ||
      typeof payload.has_next !== 'boolean' ||
      payload.page !== page
    ) {
      throw new Error('Respons daftar test case tidak sesuai format yang diharapkan.')
    }
    results.push(...payload.items.map(readTestCase))
    if (!payload.has_next) {
      break
    }
  }
  return results
}

export async function createTestCase(assignmentId, testCase) {
  let payload
  if (import.meta.env.VITE_USE_MOCK === 'true') {
    const current = readMockTestCases()
    const externalId = testCase.external_id?.trim() || generatedExternalId(current, assignmentId)
    if (current.some(
      (entry) =>
        String(entry.assignment_id) === String(assignmentId) &&
        entry.external_id === externalId,
    )) {
      await throwMockApiError(
        'test_case_external_id_exists',
        'ID eksternal test case sudah digunakan pada assignment ini.',
        409,
      )
    }
    payload = {
      id: nextMockTestCaseId(current),
      assignment_id: assignmentId,
      ...testCase,
      external_id: externalId,
    }
    writeMockTestCases([...current, payload])
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
    const current = readMockTestCases()
    const existing = current.find((entry) => String(entry.id) === String(testCaseId))
    if (!existing) {
      await throwMockApiError('not_found', 'Test case tidak ditemukan.', 404)
    }
    const externalId = testCase.external_id?.trim() || existing.external_id
    if (current.some(
      (entry) =>
        String(entry.id) !== String(testCaseId) &&
        String(entry.assignment_id) === String(existing.assignment_id) &&
        entry.external_id === externalId,
    )) {
      await throwMockApiError(
        'test_case_external_id_exists',
        'ID eksternal test case sudah digunakan pada assignment ini.',
        409,
      )
    }
    payload = { ...existing, ...testCase, external_id: externalId }
    writeMockTestCases(
      current.map((entry) => (String(entry.id) === String(testCaseId) ? payload : entry)),
    )
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
    const current = readMockTestCases()
    writeMockTestCases(current.filter((entry) => String(entry.id) !== String(testCaseId)))
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
