import { apiRequest, throwApiError, throwMockApiError } from './client.js'
import { getMockUserById, updateMockPassword, updateMockProfileName } from './auth.js'

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
    const profile = getMockUserById(user.id)
    if (!profile) {
      await throwMockApiError('not_found', 'Pengguna tidak ditemukan.', 404)
    }
    return readProfile(profile)
  }
  return requestProfile('/users/me', {}, 'Gagal memuat profil.')
}

export async function updateProfileName(name, user) {
  if (import.meta.env.VITE_USE_MOCK === 'true') {
    const updated = updateMockProfileName(user.id, name.trim())
    if (!updated) {
      await throwMockApiError('not_found', 'Pengguna tidak ditemukan.', 404)
    }
    return readProfile(updated)
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
    await updateMockPassword(user.email, currentPassword, newPassword)
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
