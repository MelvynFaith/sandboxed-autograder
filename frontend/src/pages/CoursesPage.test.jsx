import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import App from '../App.jsx'

function setUser(role) {
  localStorage.setItem('token', 'test-token')
  localStorage.setItem('refresh_token', 'test-refresh-token')
  localStorage.setItem(
    'user',
    JSON.stringify({
      id: role === 'mahasiswa' ? 'mock-student' : 'user-1',
      nama: 'Test User',
      email: 'mahasiswa@example.test',
      role,
    }),
  )
}

function paginated(items, page = 1, pageSize = 10, hasNext = false) {
  return { items, page, page_size: pageSize, has_next: hasNext }
}

function jsonResponse(status, payload) {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => payload,
  }
}

beforeEach(() => {
  localStorage.clear()
  window.history.pushState({}, '', '/courses')
  vi.stubEnv('VITE_USE_MOCK', 'true')
})

afterEach(() => {
  cleanup()
  localStorage.clear()
  vi.unstubAllEnvs()
  vi.unstubAllGlobals()
})

describe('Course management and student assignment routes', () => {
  it('lets a lecturer create a Course using mock data', async () => {
    setUser('dosen')
    render(<App />)

    expect(screen.queryByLabelText('Email dosen pengampu')).not.toBeInTheDocument()
    fireEvent.change(screen.getByLabelText('Nama'), { target: { value: 'Basis Data' } })
    fireEvent.change(screen.getByLabelText('Kode'), { target: { value: 'CS220' } })
    fireEvent.change(screen.getByLabelText('Term'), { target: { value: '2026' } })
    fireEvent.click(screen.getByRole('button', { name: 'Simpan Course' }))

    expect(await screen.findByRole('status')).toHaveTextContent('Course berhasil dibuat.')
    expect(screen.getByRole('button', { name: /CS220 — Basis Data/ })).toBeInTheDocument()
  })

  it('requires an instructor email when an administrator creates a Course', async () => {
    setUser('admin')
    render(<App />)

    const lecturerEmail = await screen.findByLabelText('Email dosen pengampu')
    expect(lecturerEmail).toBeRequired()
    fireEvent.change(screen.getByLabelText('Nama'), { target: { value: 'Basis Data' } })
    fireEvent.change(screen.getByLabelText('Kode'), { target: { value: 'CS220' } })
    fireEvent.change(screen.getByLabelText('Term'), { target: { value: '2026' } })
    fireEvent.submit(screen.getByRole('button', { name: 'Simpan Course' }).closest('form'))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Email dosen pengampu wajib diisi.',
    )
    fireEvent.change(lecturerEmail, { target: { value: 'invalid-email' } })
    fireEvent.submit(screen.getByRole('button', { name: 'Simpan Course' }).closest('form'))
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Format email dosen pengampu tidak valid.',
    )
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  it('sends lecturer_email when an administrator creates a Course', async () => {
    setUser('admin')
    vi.stubEnv('VITE_USE_MOCK', 'false')
    const createdCourse = { id: 2, code: 'CS220', name: 'Basis Data', term: '2026' }
    const fetchMock = vi.fn((url, options = {}) => {
      if (url === '/api/v1/courses' && options.method === 'POST') {
        return Promise.resolve(jsonResponse(201, createdCourse))
      }
      if (url.startsWith('/api/v1/courses/')) {
        return Promise.resolve(jsonResponse(200, paginated([])))
      }
      return Promise.resolve(jsonResponse(200, paginated([
        { id: 1, code: 'CS101', name: 'Algoritma', term: '2026' },
        createdCourse,
      ])))
    })
    vi.stubGlobal('fetch', fetchMock)
    render(<App />)

    fireEvent.change(await screen.findByLabelText('Nama'), { target: { value: 'Basis Data' } })
    fireEvent.change(screen.getByLabelText('Kode'), { target: { value: 'CS220' } })
    fireEvent.change(screen.getByLabelText('Term'), { target: { value: '2026' } })
    fireEvent.change(screen.getByLabelText('Email dosen pengampu'), {
      target: { value: 'dosen@example.test' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Simpan Course' }))

    expect(await screen.findByRole('status')).toHaveTextContent('Course berhasil dibuat.')
    const createRequest = fetchMock.mock.calls.find(
      ([url, options]) => url === '/api/v1/courses' && options.method === 'POST',
    )
    expect(JSON.parse(createRequest[1].body)).toEqual({
      name: 'Basis Data',
      code: 'CS220',
      term: '2026',
      lecturer_email: 'dosen@example.test',
    })
  })

  it('lets a lecturer enroll and remove a student from a Course', async () => {
    setUser('dosen')
    render(<App />)

    const secondCourse = await screen.findByRole('button', { name: /CS102 — Struktur Data/ })
    fireEvent.click(secondCourse)
    fireEvent.change(screen.getByLabelText('Email mahasiswa'), {
      target: { value: 'mahasiswa@example.test' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Tambah enrollment' }))

    expect(await screen.findByRole('status')).toHaveTextContent(
      'Mahasiswa berhasil ditambahkan ke Course.',
    )
    expect(screen.getByText('mahasiswa@example.test')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Keluarkan mahasiswa' }))

    expect(await screen.findByRole('status')).toHaveTextContent('Enrollment berhasil dihapus.')
    expect(screen.getByText('Belum ada mahasiswa yang terdaftar pada Course ini.')).toBeInTheDocument()
  })

  it('redirects students away from Course management and shows their assignments grouped by Course', async () => {
    setUser('mahasiswa')
    render(<App />)

    expect(await screen.findByRole('heading', { name: 'Daftar Assignment' })).toBeInTheDocument()
    expect(await screen.findByRole('heading', { name: /CS101 — Pemrograman Dasar/ })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Course' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Simpan Course' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Tambah enrollment' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Daftar' })).not.toBeInTheDocument()
    expect(window.location.pathname).toBe('/assignments')
  })

  it('loads only the student course and assignment endpoints without enrollment access', async () => {
    setUser('mahasiswa')
    vi.stubEnv('VITE_USE_MOCK', 'false')
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse(200, paginated([
        { id: 1, code: 'CS101', name: 'Algoritma', term: '2026' },
      ])))
      .mockResolvedValueOnce(jsonResponse(200, paginated([
        {
          id: 4,
          course_id: 1,
          title: 'Tugas Algoritma',
          description: 'Deskripsi',
          deadline_at: '2026-10-12T16:59:00Z',
          late_until_at: '2026-10-13T16:59:00Z',
        },
      ])))
    vi.stubGlobal('fetch', fetchMock)
    render(<App />)

    expect(await screen.findByRole('heading', { name: 'Tugas Algoritma' })).toBeInTheDocument()
    expect(fetchMock.mock.calls.map(([url]) => url)).toEqual([
      '/api/v1/courses?page=1&page_size=10',
      '/api/v1/courses/1/assignments?page=1&page_size=10',
    ])
  })

  it('shows a 409 API message when a lecturer creates a duplicate Course', async () => {
    setUser('dosen')
    vi.stubEnv('VITE_USE_MOCK', 'false')
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse(200, paginated([
        { id: 1, code: 'CS101', name: 'Algoritma', term: '2026' },
      ])))
      .mockResolvedValueOnce(jsonResponse(200, paginated([])))
      .mockResolvedValueOnce(jsonResponse(409, {
        error: { code: 'course_code_exists', message: 'Kode Course sudah digunakan.' },
      }))
    vi.stubGlobal('fetch', fetchMock)
    render(<App />)

    fireEvent.change(await screen.findByLabelText('Nama'), { target: { value: 'Algoritma Baru' } })
    fireEvent.change(screen.getByLabelText('Kode'), { target: { value: 'CS101' } })
    fireEvent.change(screen.getByLabelText('Term'), { target: { value: '2026' } })
    fireEvent.click(screen.getByRole('button', { name: 'Simpan Course' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Kode Course sudah digunakan.')
    const createRequest = fetchMock.mock.calls.find(
      ([url, options]) => url === '/api/v1/courses' && options.method === 'POST',
    )
    expect(JSON.parse(createRequest[1].body)).toEqual({
      name: 'Algoritma Baru',
      code: 'CS101',
      term: '2026',
    })
    expect(JSON.parse(createRequest[1].body)).not.toHaveProperty('lecturer_email')
  })

  it.each([
    [
      404,
      'user_not_found',
      'Dosen dengan email tersebut tidak ditemukan.',
    ],
    [
      400,
      'invalid_request',
      'Pengguna yang dipilih bukan dosen dengan peran LECTURER.',
    ],
  ])('shows administrator lecturer validation errors (%s %s)', async (status, code, message) => {
    setUser('admin')
    vi.stubEnv('VITE_USE_MOCK', 'false')
    const fetchMock = vi.fn((url, options = {}) => {
      if (url === '/api/v1/courses' && options.method === 'POST') {
        return Promise.resolve(jsonResponse(status, {
          error: { code, message },
        }))
      }
      if (url.startsWith('/api/v1/courses/')) {
        return Promise.resolve(jsonResponse(200, paginated([])))
      }
      return Promise.resolve(jsonResponse(200, paginated([
        { id: 1, code: 'CS101', name: 'Algoritma', term: '2026' },
      ])))
    })
    vi.stubGlobal('fetch', fetchMock)
    render(<App />)

    fireEvent.change(await screen.findByLabelText('Nama'), { target: { value: 'Basis Data' } })
    fireEvent.change(screen.getByLabelText('Kode'), { target: { value: 'CS220' } })
    fireEvent.change(screen.getByLabelText('Term'), { target: { value: '2026' } })
    fireEvent.change(screen.getByLabelText('Email dosen pengampu'), {
      target: { value: code === 'user_not_found' ? 'missing@example.test' : 'mahasiswa@example.test' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Simpan Course' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(message)
  })

  it('shows a 404 API message when the enrollment email is unknown', async () => {
    setUser('dosen')
    vi.stubEnv('VITE_USE_MOCK', 'false')
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse(200, paginated([
        { id: 1, code: 'CS101', name: 'Algoritma', term: '2026' },
      ])))
      .mockResolvedValueOnce(jsonResponse(200, paginated([])))
      .mockResolvedValueOnce(jsonResponse(404, {
        error: { code: 'not_found', message: 'Pengguna dengan email tersebut tidak ditemukan.' },
      }))
    vi.stubGlobal('fetch', fetchMock)
    render(<App />)

    fireEvent.change(await screen.findByLabelText('Email mahasiswa'), {
      target: { value: 'unknown@example.test' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Tambah enrollment' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Pengguna dengan email tersebut tidak ditemukan.',
    )
  })

  it('uses has_next to navigate to the next Course page', async () => {
    setUser('dosen')
    vi.stubEnv('VITE_USE_MOCK', 'false')
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse(200, paginated([
        { id: 1, code: 'CS101', name: 'Algoritma', term: '2026' },
      ], 1, 10, true)))
      .mockResolvedValueOnce(jsonResponse(200, paginated([])))
      .mockResolvedValueOnce(jsonResponse(200, paginated([
        { id: 2, code: 'CS102', name: 'Struktur Data', term: '2026' },
      ], 2, 10, false)))
      .mockResolvedValueOnce(jsonResponse(200, paginated([])))
    vi.stubGlobal('fetch', fetchMock)
    render(<App />)

    const next = (await screen.findAllByRole('button', { name: 'Berikutnya' }))[0]
    await waitFor(() => expect(next).toBeEnabled())
    fireEvent.click(next)

    expect(await screen.findByRole('button', { name: /CS102 — Struktur Data/ })).toBeInTheDocument()
    expect(fetchMock).toHaveBeenCalledWith('/api/v1/courses?page=2&page_size=10', expect.any(Object))
  })
})
