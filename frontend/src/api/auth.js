import { apiRequest } from './client.js'

const roles = {
  STUDENT: 'mahasiswa',
  LECTURER: 'dosen',
  ADMIN: 'admin',
}
const mockUsers = {
  'mahasiswa@example.test': { id: 'mock-student', name: 'Mahasiswa Demo', role: 'STUDENT' },
  'dosen@example.test': { id: 'mock-lecturer', name: 'Dosen Demo', role: 'LECTURER' },
  'admin@example.test': { id: 'mock-admin', name: 'Admin Demo', role: 'ADMIN' },
}

function readLoginResponse(payload) {
  const user = payload?.user
  if (
    typeof payload?.access_token !== 'string' ||
    !payload.access_token ||
    typeof payload?.refresh_token !== 'string' ||
    !payload.refresh_token ||
    typeof user?.id !== 'string' ||
    typeof user?.name !== 'string' ||
    typeof user?.email !== 'string' ||
    !Object.hasOwn(roles, user.role)
  ) {
    throw new Error('Respons login tidak sesuai format yang diharapkan.')
  }

  return {
    token: payload.access_token,
    refreshToken: payload.refresh_token,
    user: { id: user.id, nama: user.name, email: user.email, role: roles[user.role] },
  }
}

async function mockLogin(email, password) {
  const mockUser = mockUsers[email.toLowerCase()]
  if (!mockUser || password !== 'password123') {
    throw new Error('Email atau password tidak valid.')
  }

  return readLoginResponse({
    access_token: `mock-access-token-${mockUser.role}`,
    refresh_token: `mock-refresh-token-${mockUser.role}`,
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

export async function logout(refreshToken) {
  if (import.meta.env.VITE_USE_MOCK === 'true') {
    return
  }

  const response = await apiRequest('/auth/logout', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ refresh_token: refreshToken }),
  })
  if (!response.ok) {
    throw new Error(`Logout gagal (${response.status}).`)
  }
}
