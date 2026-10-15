const API_BASE_URL = '/api/v1'
let refreshRequest

export async function throwApiError(response, fallbackMessage) {
  let payload
  try {
    payload = await response.json()
  } catch (error) {
    if (!(error instanceof SyntaxError) && !(error instanceof TypeError)) {
      throw error
    }
  }

  const apiError = payload?.error
  if (
    typeof apiError?.code === 'string' &&
    apiError.code &&
    typeof apiError.message === 'string' &&
    apiError.message.trim()
  ) {
    throw new Error(apiError.message)
  }

  if (response.status === 429) {
    const retryAfter = response.headers?.get('Retry-After')
    if (retryAfter && /^\d+$/.test(retryAfter)) {
      throw new Error(`Terlalu banyak permintaan. Coba lagi dalam ${retryAfter} detik.`)
    }
    throw new Error('Terlalu banyak permintaan. Silakan coba lagi nanti.')
  }

  throw new Error(fallbackMessage)
}

function requestHeaders(options, token) {
  const headers = new Headers(options.headers)
  if (token) {
    headers.set('Authorization', `Bearer ${token}`)
  }
  return headers
}

async function refreshAccessToken() {
  const refreshToken = localStorage.getItem('refresh_token')
  if (!refreshToken) {
    return null
  }

  const response = await fetch(`${API_BASE_URL}/auth/refresh`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ refresh_token: refreshToken }),
  })
  if (!response.ok) {
    localStorage.removeItem('token')
    localStorage.removeItem('refresh_token')
    localStorage.removeItem('user')
    return null
  }

  const payload = await response.json()
  if (
    typeof payload?.access_token !== 'string' ||
    !payload.access_token ||
    typeof payload?.refresh_token !== 'string' ||
    !payload.refresh_token
  ) {
    throw new Error('Respons refresh token tidak sesuai format yang diharapkan.')
  }

  localStorage.setItem('token', payload.access_token)
  localStorage.setItem('refresh_token', payload.refresh_token)
  return payload.access_token
}

export async function apiRequest(path, options = {}, canRefresh = true) {
  const normalizedPath = path.startsWith('/') ? path : `/${path}`
  const token = localStorage.getItem('token')
  const response = await fetch(`${API_BASE_URL}${normalizedPath}`, {
    ...options,
    headers: requestHeaders(options, token),
  })
  if (response.status !== 401 || !canRefresh || normalizedPath === '/auth/refresh') {
    return response
  }

  refreshRequest ??= refreshAccessToken().finally(() => {
    refreshRequest = undefined
  })
  const refreshedToken = await refreshRequest
  if (!refreshedToken) {
    return response
  }

  return fetch(`${API_BASE_URL}${normalizedPath}`, {
    ...options,
    headers: requestHeaders(options, refreshedToken),
  })
}
