import { apiRequest } from './client.js'

const TERMINAL_STATUSES = new Set(['completed', 'error', 'timeout'])
const VALID_STATUSES = new Set(['queued', 'running', ...TERMINAL_STATUSES])
const POLL_INTERVAL_MS = 3000

function readStatusMessage(payload) {
  if (
    !payload ||
    typeof payload !== 'object' ||
    !VALID_STATUSES.has(payload.status) ||
    !Number.isInteger(payload.current_test) ||
    payload.current_test < 0 ||
    !Number.isInteger(payload.total_tests) ||
    payload.total_tests < 0 ||
    payload.current_test > payload.total_tests
  ) {
    throw new Error('Pesan status submission tidak sesuai format yang diharapkan.')
  }

  return {
    status: payload.status,
    current_test: payload.current_test,
    total_tests: payload.total_tests,
  }
}

async function getSubmissionStatus(submissionId) {
  const response = await apiRequest(`/submissions/${encodeURIComponent(submissionId)}`)
  if (!response.ok) {
    throw new Error(`Gagal memuat status submission (${response.status}).`)
  }

  const payload = await response.json()
  if (
    (typeof payload?.id !== 'string' && typeof payload?.id !== 'number') ||
    String(payload.id) !== String(submissionId)
  ) {
    throw new Error('Respons status submission tidak sesuai format yang diharapkan.')
  }

  return readStatusMessage(payload)
}

function createWebSocketUrl(submissionId, token) {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const url = new URL(`/ws/v1/submissions/${encodeURIComponent(submissionId)}`, window.location.href)
  url.protocol = protocol
  url.searchParams.set('token', token)
  return url.toString()
}

function startMockStatus(onStatus) {
  const updates = [
    { status: 'queued', current_test: 0, total_tests: 3 },
    { status: 'running', current_test: 1, total_tests: 3 },
    { status: 'running', current_test: 2, total_tests: 3 },
    { status: 'completed', current_test: 3, total_tests: 3 },
  ]
  let index = 0

  onStatus(updates[index])
  const timer = window.setInterval(() => {
    index += 1
    onStatus(updates[index])
    if (index === updates.length - 1) {
      window.clearInterval(timer)
    }
  }, 600)

  return () => window.clearInterval(timer)
}

export function subscribeToSubmissionStatus(submissionId, token, handlers) {
  const { onStatus, onFallback, onError } = handlers

  if (import.meta.env.VITE_USE_MOCK === 'true') {
    return startMockStatus(onStatus)
  }

  let disposed = false
  let terminal = false
  let fallbackStarted = false
  let pollingTimer
  let socket

  function stop() {
    if (pollingTimer !== undefined) {
      window.clearInterval(pollingTimer)
      pollingTimer = undefined
    }
    if (socket && socket.readyState < WebSocket.CLOSING) {
      socket.close()
    }
  }

  function receiveStatus(payload) {
    const status = readStatusMessage(payload)
    onStatus(status)
    if (TERMINAL_STATUSES.has(status.status)) {
      terminal = true
      stop()
    }
  }

  async function poll() {
    try {
      receiveStatus(await getSubmissionStatus(submissionId))
    } catch (error) {
      if (!disposed && !terminal) {
        onError(error.message)
      }
    }
  }

  function startPolling() {
    if (disposed || terminal || fallbackStarted) {
      return
    }
    fallbackStarted = true
    onFallback('Koneksi real-time terputus. Status diperbarui melalui polling setiap 3 detik.')
    poll()
    pollingTimer = window.setInterval(poll, POLL_INTERVAL_MS)
  }

  try {
    if (!token) {
      throw new Error('Token autentikasi tidak tersedia untuk koneksi status.')
    }
    socket = new WebSocket(createWebSocketUrl(submissionId, token))
  } catch {
    startPolling()
  }

  if (socket) {
    socket.onmessage = (event) => {
      if (disposed || terminal) {
        return
      }
      try {
        receiveStatus(JSON.parse(event.data))
      } catch (error) {
        onError(error.message)
        startPolling()
        socket.close()
      }
    }
    socket.onerror = () => {
      startPolling()
      if (!terminal && socket.readyState < WebSocket.CLOSING) {
        socket.close()
      }
    }
    socket.onclose = () => {
      if (!disposed && !terminal) {
        startPolling()
      }
    }
  }

  return () => {
    disposed = true
    stop()
  }
}
