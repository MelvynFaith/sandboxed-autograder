import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  createTestCase,
  deleteTestCase,
  exportTestCasesYaml,
  getTestCases,
  importTestCasesYaml,
  updateTestCase,
} from './testcases.js'

beforeEach(() => {
  localStorage.clear()
  localStorage.setItem('token', 'yaml-token')
  vi.stubEnv('VITE_USE_MOCK', 'false')
})

afterEach(() => {
  localStorage.clear()
  vi.unstubAllEnvs()
  vi.unstubAllGlobals()
})

describe('test-case YAML API', () => {
  it('downloads YAML from the assignment export endpoint', async () => {
    const blob = new Blob(['schema_version: 1\n'])
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      headers: new Headers({
        'Content-Disposition': 'attachment; filename="testcases-assignment-42.yaml"',
      }),
      blob: async () => blob,
    })
    vi.stubGlobal('fetch', fetchMock)

    await expect(exportTestCasesYaml('42')).resolves.toEqual({
      blob,
      filename: 'testcases-assignment-42.yaml',
    })
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/assignments/42/testcases/export',
      expect.objectContaining({ headers: expect.any(Headers) }),
    )
    expect(fetchMock.mock.calls[0][1].headers.get('Authorization')).toMatch(/^Bearer .+/)
  })

  it('posts YAML to the assignment import endpoint and validates the result', async () => {
    const document = 'schema_version: 1\ntest_cases: []\n'
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ created: 2, updated: 1 }),
    })
    vi.stubGlobal('fetch', fetchMock)

    await expect(importTestCasesYaml('assignment/1', document)).resolves.toEqual({
      created: 2,
      updated: 1,
    })
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/assignments/assignment%2F1/testcases/import',
      expect.objectContaining({
        method: 'POST',
        headers: expect.any(Headers),
        body: document,
      }),
    )
    expect(fetchMock.mock.calls[0][1].headers.get('Content-Type')).toBe('application/yaml')
    expect(fetchMock.mock.calls[0][1].headers.get('Authorization')).toMatch(/^Bearer .+/)
  })

  it('shows the backend validation message for invalid YAML', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
      json: async () => ({
        error: { code: 'invalid_yaml', message: 'test_cases[0].weight harus positif.' },
      }),
    }))

    await expect(
      importTestCasesYaml('assignment-1', 'schema_version: 1\ntest_cases: []\n'),
    ).rejects.toThrow('test_cases[0].weight harus positif.')
  })

  it('rejects an invalid import response instead of reporting success', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ created: -1, updated: 0 }),
    }))

    await expect(
      importTestCasesYaml('assignment-1', 'schema_version: 1\ntest_cases: []\n'),
    ).rejects.toThrow('Respons impor YAML tidak sesuai format yang diharapkan.')
  })
})

describe('test-case CRUD API', () => {
  const testCase = {
    external_id: 'tc_01',
    input: '2 3',
    expected_output: '5',
    weight: 10,
    is_hidden: true,
    match_mode: 'WHITESPACE_INSENSITIVE',
  }

  it('reads all pages of test cases and preserves the entity fields', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          items: [{ id: 1, assignment_id: 42, ...testCase }],
          page: 1,
          page_size: 50,
          has_next: true,
        }),
      })
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          items: [{
            id: 2,
            assignment_id: 42,
            ...testCase,
            external_id: 'tc_02',
            is_hidden: false,
          }],
          page: 2,
          page_size: 50,
          has_next: false,
        }),
      })
    vi.stubGlobal('fetch', fetchMock)

    await expect(getTestCases('42')).resolves.toEqual([
      {
        id: '1',
        assignmentId: '42',
        externalId: 'tc_01',
        input: '2 3',
        expectedOutput: '5',
        weight: 10,
        hidden: true,
        matchMode: 'WHITESPACE_INSENSITIVE',
      },
      {
        id: '2',
        assignmentId: '42',
        externalId: 'tc_02',
        input: '2 3',
        expectedOutput: '5',
        weight: 10,
        hidden: false,
        matchMode: 'WHITESPACE_INSENSITIVE',
      },
    ])
    expect(fetchMock.mock.calls.map(([url]) => url)).toEqual([
      '/api/v1/assignments/42/testcases?page=1&page_size=50',
      '/api/v1/assignments/42/testcases?page=2&page_size=50',
    ])
  })

  it('creates, updates, and deletes test cases with the backend request shapes', async () => {
    const saved = { id: 7, assignment_id: 42, ...testCase }
    const updated = { ...saved, weight: 12, is_hidden: false }
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({ ok: true, status: 201, json: async () => saved })
      .mockResolvedValueOnce({ ok: true, status: 200, json: async () => updated })
      .mockResolvedValueOnce({ ok: true, status: 204 })
    vi.stubGlobal('fetch', fetchMock)

    await expect(createTestCase('42', testCase)).resolves.toMatchObject({
      id: '7',
      assignmentId: '42',
      externalId: 'tc_01',
    })
    await expect(updateTestCase('7', { ...testCase, weight: 12, is_hidden: false }))
      .resolves.toMatchObject({ id: '7', weight: 12, hidden: false })
    await expect(deleteTestCase('7')).resolves.toBeUndefined()

    expect(fetchMock.mock.calls.map(([url, options]) => [url, options.method, options.body]))
      .toEqual([
        ['/api/v1/assignments/42/testcases', 'POST', JSON.stringify(testCase)],
        [
          '/api/v1/testcases/7',
          'PUT',
          JSON.stringify({ ...testCase, weight: 12, is_hidden: false }),
        ],
        ['/api/v1/testcases/7', 'DELETE', undefined],
      ])
  })

  it('surfaces duplicate external IDs as a conflict', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      status: 409,
      json: async () => ({
        error: {
          code: 'test_case_external_id_exists',
          message: 'ID eksternal test case sudah digunakan pada assignment ini.',
        },
      }),
    }))

    await expect(createTestCase('42', testCase)).rejects.toThrow(
      'ID eksternal test case sudah digunakan pada assignment ini.',
    )
  })

  it('generates external IDs and persists mock test-case CRUD', async () => {
    vi.stubEnv('VITE_USE_MOCK', 'true')

    const created = await createTestCase('42', { ...testCase, external_id: '' })
    expect(created.externalId).toBe('tc_01')
    await expect(getTestCases('42')).resolves.toEqual([created])

    const updated = await updateTestCase(created.id, {
      ...testCase,
      weight: 12,
      external_id: '',
    })
    expect(updated).toMatchObject({ externalId: 'tc_01', weight: 12 })
    await deleteTestCase(created.id)
    await expect(getTestCases('42')).resolves.toEqual([])
  })
})
