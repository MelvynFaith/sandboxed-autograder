import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { subscribeToSubmissionStatus } from './submissionStatus.js'

let sockets

class MockWebSocket {
  static CONNECTING = 0
  static OPEN = 1
  static CLOSING = 2
  static CLOSED = 3

  constructor(url) {
    this.url = url
    this.readyState = MockWebSocket.OPEN
    this.close = vi.fn(() => {
      this.readyState = MockWebSocket.CLOSED
      this.onclose?.()
    })
    sockets.push(this)
  }
}

beforeEach(() => {
  sockets = []
  localStorage.setItem('token', 'test-token')
  localStorage.setItem('refresh_token', 'refresh-token')
  vi.stubGlobal('WebSocket', MockWebSocket)
  vi.stubEnv('VITE_USE_MOCK', 'false')
})

afterEach(() => {
  localStorage.clear()
  vi.unstubAllEnvs()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

describe('subscribeToSubmissionStatus', () => {
  it('requests a one-time ticket and reads WebSocket status messages', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ ticket: 'one-time-ticket' }),
    }))
    const onStatus = vi.fn()
    const cleanup = subscribeToSubmissionStatus('submission-1', {
      onStatus,
      onFallback: vi.fn(),
      onError: vi.fn(),
    })

    await vi.waitFor(() => expect(sockets).toHaveLength(1))
    expect(new URL(sockets[0].url).searchParams.get('ticket')).toBe('one-time-ticket')
    expect(new URL(sockets[0].url).searchParams.has('token')).toBe(false)
    expect(new URL(sockets[0].url).pathname).toBe('/ws/v1/submissions/submission-1')

    sockets[0].onmessage({
      data: JSON.stringify({ status: 'RUNNING', current_test: 1, total_tests: 2 }),
    })
    expect(onStatus).toHaveBeenCalledWith({
      status: 'RUNNING',
      current_test: 1,
      total_tests: 2,
    })
    cleanup()
  })

  it('falls back to polling when the WebSocket closes unexpectedly', async () => {
    const onFallback = vi.fn()
    const onStatus = vi.fn()
    const onError = vi.fn()
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({ ok: true, json: async () => ({ ticket: 'ticket' }) })
      .mockResolvedValue({ ok: true, json: async () => ({
          id: 'submission-1',
          status: 'RUNNING',
        }) })
    vi.stubGlobal('fetch', fetchMock)
    const cleanup = subscribeToSubmissionStatus('submission-1', {
      onStatus,
      onFallback,
      onError,
    })

    await vi.waitFor(() => expect(sockets).toHaveLength(1))
    sockets[0].onclose()

    expect(onError).not.toHaveBeenCalled()
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/submissions/submission-1',
      expect.objectContaining({ headers: expect.any(Headers) }),
    )
    expect(onFallback).toHaveBeenCalledWith(
      'Koneksi real-time terputus. Status diperbarui melalui polling setiap 3 detik.',
    )
    await vi.waitFor(() => {
      expect(onStatus).toHaveBeenCalledWith({
        status: 'RUNNING',
        current_test: null,
        total_tests: null,
      })
    })
    expect(onError).not.toHaveBeenCalled()
    cleanup()
  })

  it('stops polling status updates after a terminal status', async () => {
    const onStatus = vi.fn()
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({ ok: true, json: async () => ({ ticket: 'ticket' }) })
      .mockResolvedValue({ ok: true, json: async () => ({
          id: 'submission-1',
          status: 'COMPLETED',
        }) })
    vi.stubGlobal('fetch', fetchMock)
    const cleanup = subscribeToSubmissionStatus('submission-1', {
      onStatus,
      onFallback: vi.fn(),
      onError: vi.fn(),
    })

    await vi.waitFor(() => expect(sockets).toHaveLength(1))
    sockets[0].onclose()

    await vi.waitFor(() => {
      expect(onStatus).toHaveBeenCalledWith({
        status: 'COMPLETED',
        current_test: null,
        total_tests: null,
      })
    })
    cleanup()
    expect(sockets[0].close).toHaveBeenCalled()
  })

  it('maps the SRS FAILED status to a failed UI state', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ ticket: 'ticket' }),
    }))
    const onStatus = vi.fn()
    const cleanup = subscribeToSubmissionStatus('submission-1', {
      onStatus,
      onFallback: vi.fn(),
      onError: vi.fn(),
    })

    await vi.waitFor(() => expect(sockets).toHaveLength(1))
    sockets[0].onmessage({
      data: JSON.stringify({ status: 'FAILED', current_test: 1, total_tests: 2 }),
    })

    expect(onStatus).toHaveBeenCalledWith({
      status: 'FAILED',
      current_test: 1,
      total_tests: 2,
    })
    cleanup()
  })

  it.each(['ERROR', 'TIMEOUT'])(
    'rejects %s because it is not a Submission status',
    async (status) => {
      vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({ ticket: 'ticket' }),
      }))
      const onError = vi.fn()
      const cleanup = subscribeToSubmissionStatus('submission-1', {
        onStatus: vi.fn(),
        onFallback: vi.fn(),
        onError,
      })

      await vi.waitFor(() => expect(sockets).toHaveLength(1))
      sockets[0].onmessage({
        data: JSON.stringify({ status, current_test: 1, total_tests: 2 }),
      })

      expect(onError).toHaveBeenCalledWith(
        'Pesan status submission tidak sesuai format yang diharapkan.',
      )
      cleanup()
    },
  )

  it('simulates queued, running, and completed statuses in mock mode', () => {
    vi.stubEnv('VITE_USE_MOCK', 'true')
    vi.useFakeTimers()
    const onStatus = vi.fn()
    const cleanup = subscribeToSubmissionStatus('submission-1', {
      onStatus,
      onFallback: vi.fn(),
      onError: vi.fn(),
    })

    expect(onStatus).toHaveBeenNthCalledWith(1, {
      status: 'QUEUED',
      current_test: 0,
      total_tests: 3,
    })
    vi.advanceTimersByTime(1800)
    expect(onStatus).toHaveBeenCalledWith({
      status: 'RUNNING',
      current_test: 2,
      total_tests: 3,
    })
    vi.advanceTimersByTime(600)
    expect(onStatus).toHaveBeenLastCalledWith({
      status: 'COMPLETED',
      current_test: 3,
      total_tests: 3,
    })
    cleanup()
    vi.useRealTimers()
  })
})
