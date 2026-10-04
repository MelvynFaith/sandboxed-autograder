const API_BASE_URL = '/api/v1'

export async function apiRequest(path, options = {}) {
  const token = localStorage.getItem('token')
  const headers = new Headers(options.headers)

  if (token) {
    headers.set('Authorization', `Bearer ${token}`)
  }

  const normalizedPath = path.startsWith('/') ? path : `/${path}`
  return fetch(`${API_BASE_URL}${normalizedPath}`, { ...options, headers })
}
