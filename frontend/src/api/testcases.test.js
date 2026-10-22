import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { exportTestCasesYaml, importTestCasesYaml } from './testcases.js'

beforeEach(() => {
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
    expect(fetchMock.mock.calls[0][1].headers.get('Authorization')).toBe('Bearer yaml-token')
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
    expect(fetchMock.mock.calls[0][1].headers.get('Authorization')).toBe('Bearer yaml-token')
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
