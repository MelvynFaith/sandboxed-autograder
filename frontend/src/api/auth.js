import { apiRequest, throwApiError, throwMockApiError } from './client.js'

const roles = {
  STUDENT: 'mahasiswa',
  LECTURER: 'dosen',
  ADMIN: 'admin',
}
const MOCK_USERS_KEY = 'sandboxed_autograder_mock_users'
const initialMockUsers = {
  'mahasiswa@example.test': { id: 'mock-student', name: 'Mahasiswa Demo', email: 'mahasiswa@example.test', role: 'STUDENT', password: 'password123' },
  'dosen@example.test': { id: 'mock-lecturer', name: 'Dosen Demo', email: 'dosen@example.test', role: 'LECTURER', password: 'password123' },
  'admin@example.test': { id: 'mock-admin', name: 'Admin Demo', email: 'admin@example.test', role: 'ADMIN', password: 'password123' },
}
const seededTokenSuffix = {
  'mock-student': 'STUDENT',
  'mock-lecturer': 'LECTURER',
  'mock-admin': 'ADMIN',
}

function readMockUsers() {
  const serialized = localStorage.getItem(MOCK_USERS_KEY)
  if (!serialized) {
    return Object.fromEntries(
      Object.entries(initialMockUsers).map(([email, user]) => [email, { ...user }]),
    )
  }

  const users = JSON.parse(serialized)
  if (!users || typeof users !== 'object' || Array.isArray(users)) {
    throw new Error('Data pengguna mock tidak valid.')
  }
  return users
}

function saveMockUsers(users) {
  localStorage.setItem(MOCK_USERS_KEY, JSON.stringify(users))
}

function findMockUser(users, predicate) {
  return Object.values(users).find(predicate)
}

function publicMockUser(user) {
  return { id: user.id, name: user.name, email: user.email, role: user.role }
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
  const mockUser = readMockUsers()[email.trim().toLowerCase()]
  if (!mockUser || password !== mockUser.password) {
    await throwMockApiError('invalid_credentials', 'Email atau password tidak valid.', 401)
  }

  const tokenSuffix = seededTokenSuffix[mockUser.id] ?? mockUser.id
  return readLoginResponse({
    access_token: `mock-access-token-${tokenSuffix}`,
    refresh_token: `mock-refresh-token-${tokenSuffix}`,
    user: { ...mockUser, email: email.trim().toLowerCase() },
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
    const users = readMockUsers()
    if (users[normalizedEmail]) {
      await throwMockApiError('email_already_exists', 'Email sudah terdaftar.', 409)
    }

    let id
    do {
      id = `mock-student-${Date.now()}-${Math.random().toString(36).slice(2)}`
    } while (findMockUser(users, (user) => user.id === id))
    users[normalizedEmail] = {
      id,
      name: name.trim(),
      email: normalizedEmail,
      role: 'STUDENT',
      password,
    }
    saveMockUsers(users)
    return publicMockUser(users[normalizedEmail])
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

export function getMockUserById(id) {
  const mockUser = findMockUser(readMockUsers(), (user) => String(user.id) === String(id))
  return mockUser ? publicMockUser(mockUser) : null
}

export function updateMockProfileName(id, name) {
  const users = readMockUsers()
  const mockUser = findMockUser(users, (user) => String(user.id) === String(id))
  if (!mockUser) {
    return null
  }

  mockUser.name = name
  saveMockUsers(users)
  return publicMockUser(mockUser)
}

export async function updateMockPassword(email, currentPassword, newPassword) {
  const users = readMockUsers()
  const mockUser = users[email.trim().toLowerCase()]
  if (!mockUser || mockUser.password !== currentPassword) {
    await throwMockApiError('invalid_password', 'Kata sandi lama tidak sesuai.', 400)
  }
  mockUser.password = newPassword
  saveMockUsers(users)
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
