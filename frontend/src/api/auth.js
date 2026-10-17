import { apiRequest, throwApiError } from './client.js'

const roles = {
  STUDENT: 'mahasiswa',
  LECTURER: 'dosen',
  ADMIN: 'admin',
}
const mockUsers = {
  'mahasiswa@example.test': { id: 'mock-student', name: 'Mahasiswa Demo', role: 'STUDENT', password: 'password123' },
  'dosen@example.test': { id: 'mock-lecturer', name: 'Dosen Demo', role: 'LECTURER', password: 'password123' },
  'admin@example.test': { id: 'mock-admin', name: 'Admin Demo', role: 'ADMIN', password: 'password123' },
}

function readLoginResponse(payload) {
  const user = payload?.user
  if (
    typeof payload?.access_token !== 'string' ||
    !payload.access_token ||
    typeof payload?.refresh_token !== 'string' ||
    !payload.refresh_token ||
    (typeof user?.id !== 'string' && typeof user?.id !== 'number') ||
    typeof user?.name !== 'string' ||
    typeof user?.email !== 'string' ||
    !Object.hasOwn(roles, user.role)
  ) {
    throw new Error('Respons login tidak sesuai format yang diharapkan.')
  }

  return {
    token: payload.access_token,
    refreshToken: payload.refresh_token,
    user: { id: String(user.id), nama: user.name, email: user.email, role: roles[user.role] },
  }
}

async function mockLogin(email, password) {
  const mockUser = mockUsers[email.toLowerCase()]
  if (!mockUser || password !== mockUser.password) {
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
    await throwApiError(response, `Login gagal (${response.status}). Periksa email dan password.`)
  }

  return readLoginResponse(await response.json())
}

export async function register(name, email, password) {
  const body = { name, email, password }
  if (import.meta.env.VITE_USE_MOCK === 'true') {
    const normalizedEmail = email.trim().toLowerCase()
    if (mockUsers[normalizedEmail]) {
      throw new Error('Email sudah terdaftar. Gunakan email lain.')
    }

    mockUsers[normalizedEmail] = {
      id: `mock-student-${Date.now()}`,
      name,
      role: 'STUDENT',
      password,
    }
    return { id: mockUsers[normalizedEmail].id, name, email: normalizedEmail, role: 'STUDENT' }
  }

  const response = await apiRequest('/auth/register', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  if (!response.ok) {
    await throwApiError(response, `Registrasi gagal (${response.status}).`)
  }

  return readUserResponse(await response.json())
}

export function updateMockPassword(email, currentPassword, newPassword) {
  const mockUser = mockUsers[email.toLowerCase()]
  if (!mockUser || mockUser.password !== currentPassword) {
    throw new Error('Kata sandi lama tidak sesuai.')
  }
  mockUser.password = newPassword
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
    await throwApiError(response, `Logout gagal (${response.status}).`)
  }
}

function readUserResponse(payload) {
  const user = payload?.user ?? payload
  if (
    (typeof user?.id !== 'string' && typeof user?.id !== 'number') ||
    typeof user?.name !== 'string' ||
    typeof user?.email !== 'string' ||
    !Object.hasOwn(roles, user?.role)
  ) {
    throw new Error('Respons pengguna tidak sesuai format yang diharapkan.')
  }
  return {
    id: String(user.id),
    name: user.name,
    email: user.email,
    role: user.role,
  }
}
