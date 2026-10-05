import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import App from '../App.jsx'

function setUser(role) {
  localStorage.setItem('token', 'test-token')
  localStorage.setItem(
    'user',
    JSON.stringify({
      id: 'user-1',
      nama: 'Test User',
      email: 'test@example.test',
      role,
    }),
  )
}

beforeEach(() => {
  localStorage.clear()
  window.history.pushState({}, '', '/assignments')
  vi.stubEnv('VITE_USE_MOCK', 'true')
})

afterEach(() => {
  cleanup()
  localStorage.clear()
  vi.unstubAllEnvs()
})

describe('student assignment submissions', () => {
  it('shows assignments from the API', async () => {
    setUser('mahasiswa')
    render(<App />)

    expect(await screen.findByRole('heading', { name: 'Dasar Algoritma' })).toBeInTheDocument()
    expect(screen.getByText(/Buat program Python/)).toBeInTheDocument()
    expect(screen.getAllByText(/Deadline:/)).toHaveLength(2)
    expect(screen.getByText('12 Oktober 2026 pukul 23.59')).toBeInTheDocument()
  })

  it('uploads a valid Python file and shows success', async () => {
    setUser('mahasiswa')
    render(<App />)
    await screen.findByRole('heading', { name: 'Dasar Algoritma' })

    const file = new File(['print("hello")'], 'solution.py', { type: 'text/x-python' })
    fireEvent.change(screen.getByLabelText('Kode Python untuk Dasar Algoritma'), {
      target: { files: [file] },
    })
    fireEvent.click(screen.getAllByRole('button', { name: 'Upload kode Python' })[0])

    expect(await screen.findByRole('status')).toHaveTextContent(
      'Kode berhasil diunggah. Status: queued.',
    )
  })

  it('rejects files that do not have a Python extension', async () => {
    setUser('mahasiswa')
    render(<App />)
    await screen.findByRole('heading', { name: 'Dasar Algoritma' })

    const file = new File(['not python'], 'solution.txt', { type: 'text/plain' })
    fireEvent.change(screen.getByLabelText('Kode Python untuk Dasar Algoritma'), {
      target: { files: [file] },
    })

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Pilih berkas dengan ekstensi .py.',
    )
    expect(screen.getAllByRole('button', { name: 'Upload kode Python' })[0]).toBeDisabled()
  })

  it('rejects Python files larger than 1 MiB', async () => {
    setUser('mahasiswa')
    render(<App />)
    await screen.findByRole('heading', { name: 'Dasar Algoritma' })

    const file = new File([new Uint8Array(1024 * 1024 + 1)], 'large.py')
    fireEvent.change(screen.getByLabelText('Kode Python untuk Dasar Algoritma'), {
      target: { files: [file] },
    })

    expect(await screen.findByRole('alert')).toHaveTextContent('Ukuran berkas maksimal 1 MiB.')
    expect(screen.getAllByRole('button', { name: 'Upload kode Python' })[0]).toBeDisabled()
  })

  it('does not show upload controls to non-student roles', async () => {
    setUser('dosen')
    render(<App />)

    expect(await screen.findByRole('heading', { name: 'Dasar Algoritma' })).toBeInTheDocument()
    expect(screen.queryByLabelText('Kode Python untuk Dasar Algoritma')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Upload kode Python' })).not.toBeInTheDocument()
  })
})
