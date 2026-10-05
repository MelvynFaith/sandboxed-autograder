import { apiRequest } from './client.js'

const mockAssignments = [
  {
    id: 'assignment-1',
    judul: 'Dasar Algoritma',
    deskripsi: 'Buat program Python untuk menyelesaikan soal algoritma dasar.',
    deadline: '2026-10-12T23:59:00+07:00',
  },
  {
    id: 'assignment-2',
    judul: 'Struktur Data',
    deskripsi: 'Implementasikan struktur data yang diminta menggunakan Python.',
    deadline: '2026-10-19T23:59:00+07:00',
  },
]

function readAssignmentsResponse(payload) {
  if (
    !Array.isArray(payload) ||
    payload.some(
      (assignment) =>
        !assignment ||
        typeof assignment !== 'object' ||
        (typeof assignment.id !== 'string' && typeof assignment.id !== 'number') ||
        typeof assignment.judul !== 'string' ||
        typeof assignment.deskripsi !== 'string' ||
        typeof assignment.deadline !== 'string' ||
        !Number.isFinite(Date.parse(assignment.deadline)),
    )
  ) {
    throw new Error('Respons daftar assignment tidak sesuai format yang diharapkan.')
  }

  return payload
}

export async function getAssignments() {
  if (import.meta.env.VITE_USE_MOCK === 'true') {
    return readAssignmentsResponse(mockAssignments)
  }

  const response = await apiRequest('/assignments')
  if (!response.ok) {
    throw new Error(`Gagal memuat assignment (${response.status}).`)
  }

  return readAssignmentsResponse(await response.json())
}
