const API_BASE_URL = '/api/v1'
export const AUTH_SESSION_INVALIDATED = 'auth:session-invalidated'
let refreshRequest

function invalidateStoredSession() {
  localStorage.removeItem('token')
  localStorage.removeItem('refresh_token')
  localStorage.removeItem('user')
  window.dispatchEvent(new Event(AUTH_SESSION_INVALIDATED))
}

// Retry-After of a 429 in whole seconds, or null. The API sends it with every rate-limit refusal
// (FR-14), so the dashboard can count down instead of only quoting the number.
function retryAfterSeconds(response) {
  if (response.status !== 429) {
    return null
  }
  const value = response.headers?.get('Retry-After')
  return value && /^\d+$/.test(value) ? Number(value) : null
}

function apiFailure(message, response) {
  const error = new Error(message)
  error.status = response.status
  error.retryAfter = retryAfterSeconds(response)
  return error
}

export async function throwApiError(response, fallbackMessage, statusMessages = {}) {
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
    throw apiFailure(apiError.message, response)
  }

  const statusMessage = statusMessages[response.status]
  if (statusMessage) {
    throw apiFailure(
      typeof statusMessage === 'function' ? statusMessage(response) : statusMessage,
      response,
    )
  }

  if (response.status === 429) {
    const retryAfter = response.headers?.get('Retry-After')
    if (retryAfter && /^\d+$/.test(retryAfter)) {
      throw apiFailure(`Terlalu banyak permintaan. Coba lagi dalam ${retryAfter} detik.`, response)
    }
    throw apiFailure('Terlalu banyak permintaan. Silakan coba lagi nanti.', response)
  }

  throw apiFailure(fallbackMessage, response)
}

export async function throwMockApiError(code, message, status = 400) {
  await throwApiError({
    ok: false,
    status,
    json: async () => ({ error: { code, message } }),
  }, message)
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
    invalidateStoredSession()
    return null
  }

  const payload = await response.json()
  if (
    typeof payload?.access_token !== 'string' ||
    !payload.access_token ||
    (payload.refresh_token !== undefined &&
      (typeof payload.refresh_token !== 'string' || !payload.refresh_token))
  ) {
    invalidateStoredSession()
    throw new Error('Respons refresh token tidak sesuai format yang diharapkan.')
  }

  localStorage.setItem('token', payload.access_token)
  if (payload.refresh_token !== undefined) {
    localStorage.setItem('refresh_token', payload.refresh_token)
  }
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
