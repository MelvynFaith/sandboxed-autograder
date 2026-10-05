import { apiRequest } from './client.js'

function isIdentifier(value) {
  return typeof value === 'string' || typeof value === 'number'
}

function readCreatedAssignment(payload) {
  if (
    !payload ||
    typeof payload !== 'object' ||
    !isIdentifier(payload.id) ||
    typeof payload.judul !== 'string' ||
    typeof payload.deskripsi !== 'string' ||
    typeof payload.deadline !== 'string' ||
    !Number.isFinite(Date.parse(payload.deadline))
  ) {
    throw new Error('Respons assignment tidak sesuai format yang diharapkan.')
  }

  return {
    id: String(payload.id),
    judul: payload.judul,
    deskripsi: payload.deskripsi,
    deadline: payload.deadline,
  }
}

export async function createLecturerAssignment(assignment) {
  let payload

  if (import.meta.env.VITE_USE_MOCK === 'true') {
    payload = {
      id: 'assignment-created-1',
      judul: assignment.judul,
      deskripsi: assignment.deskripsi,
      deadline: assignment.deadline,
    }
  } else {
    const response = await apiRequest('/assignments', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(assignment),
    })
    if (!response.ok) {
      throw new Error(`Gagal membuat assignment (${response.status}).`)
    }
    payload = await response.json()
  }

  return readCreatedAssignment(payload)
}
