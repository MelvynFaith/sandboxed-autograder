import { apiRequest, throwApiError } from './client.js'
import { updateMockPassword } from './auth.js'

const mockProfiles = new Map()

function readProfile(payload) {
  const user = payload?.user ?? payload
  if (
    (typeof user?.id !== 'string' && typeof user?.id !== 'number') ||
    typeof user?.name !== 'string' ||
    typeof user?.email !== 'string' ||
    !['STUDENT', 'LECTURER', 'ADMIN'].includes(user?.role)
  ) {
    throw new Error('Respons profil tidak sesuai format yang diharapkan.')
  }
  return {
    id: String(user.id),
    name: user.name,
    email: user.email,
    role: user.role,
  }
}

async function requestProfile(path, options, fallback) {
  const response = await apiRequest(path, options)
  if (!response.ok) {
    await throwApiError(response, fallback)
  }
  return readProfile(await response.json())
}

export async function getProfile(user) {
  if (import.meta.env.VITE_USE_MOCK === 'true') {
    const profile = mockProfiles.get(String(user.id)) ?? {
      id: user.id,
      name: user.nama,
      email: user.email,
      role: ({ mahasiswa: 'STUDENT', dosen: 'LECTURER', admin: 'ADMIN' })[user.role],
    }
    mockProfiles.set(String(user.id), profile)
    return readProfile(profile)
  }
  return requestProfile('/users/me', {}, 'Gagal memuat profil.')
}

export async function updateProfileName(name, user) {
  if (import.meta.env.VITE_USE_MOCK === 'true') {
    const current = await getProfile(user)
    const updated = { ...current, name }
    mockProfiles.set(String(user.id), updated)
    return updated
  }
  return requestProfile(
    '/users/me',
    {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name }),
    },
    'Gagal memperbarui profil.',
  )
}

export async function updatePassword(currentPassword, newPassword, user) {
  if (import.meta.env.VITE_USE_MOCK === 'true') {
    updateMockPassword(user.email, currentPassword, newPassword)
    return
  }

  const response = await apiRequest('/users/me/password', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      current_password: currentPassword,
      new_password: newPassword,
    }),
  })
  if (!response.ok) {
    await throwApiError(response, `Gagal mengganti kata sandi (${response.status}).`)
  }
}
