import { apiRequest } from './client.js'

const roles = ['mahasiswa', 'dosen', 'admin']
const mockUsers = {
  'mahasiswa@example.test': { id: 'mock-student', nama: 'Mahasiswa Demo', role: 'mahasiswa' },
  'dosen@example.test': { id: 'mock-lecturer', nama: 'Dosen Demo', role: 'dosen' },
  'admin@example.test': { id: 'mock-admin', nama: 'Admin Demo', role: 'admin' },
}

function readLoginResponse(payload) {
  const user = payload?.user
  if (
    typeof payload?.token !== 'string' ||
    !payload.token ||
    typeof user?.id !== 'string' ||
    typeof user?.nama !== 'string' ||
    typeof user?.email !== 'string' ||
    !roles.includes(user.role)
  ) {
    throw new Error('Respons login tidak sesuai format yang diharapkan.')
  }

  return { token: payload.token, user }
}

async function mockLogin(email, password) {
  const mockUser = mockUsers[email.toLowerCase()]
  if (!mockUser || password !== 'password123') {
    throw new Error('Email atau password tidak valid.')
  }

  return readLoginResponse({
    token: `mock-token-${mockUser.role}`,
    user: { ...mockUser, email },
  })
}

export async function login(email, password) {
  if (import.meta.env.VITE_USE_MOCK === 'true') {
    return mockLogin(email, password)
  }

  const response = await apiRequest('/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
  })

  if (!response.ok) {
    throw new Error(`Login gagal (${response.status}). Periksa email dan password.`)
  }

  return readLoginResponse(await response.json())
}
