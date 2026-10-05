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
  it('connects with the assumed query token and reads WebSocket status messages', () => {
    const onStatus = vi.fn()
    const cleanup = subscribeToSubmissionStatus('submission-1', 'jwt-value', {
      onStatus,
      onFallback: vi.fn(),
      onError: vi.fn(),
    })

    expect(new URL(sockets[0].url).searchParams.get('token')).toBe('jwt-value')
    expect(new URL(sockets[0].url).pathname).toBe('/ws/v1/submissions/submission-1')

    sockets[0].onmessage({
      data: JSON.stringify({ status: 'running', current_test: 1, total_tests: 2 }),
    })
    expect(onStatus).toHaveBeenCalledWith({
      status: 'running',
      current_test: 1,
      total_tests: 2,
    })
    cleanup()
  })

  it('falls back to polling when the WebSocket closes unexpectedly', async () => {
    const onFallback = vi.fn()
    const onStatus = vi.fn()
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        id: 'submission-1',
        status: 'running',
        current_test: 1,
        total_tests: 3,
      }),
    })
    vi.stubGlobal('fetch', fetchMock)
    const cleanup = subscribeToSubmissionStatus('submission-1', 'jwt-value', {
      onStatus,
      onFallback,
      onError: vi.fn(),
    })

    sockets[0].onclose()

    await vi.waitFor(() => {
      expect(onStatus).toHaveBeenCalledWith({
        status: 'running',
        current_test: 1,
        total_tests: 3,
      })
    })
    expect(fetchMock).toHaveBeenCalledWith('/api/v1/submissions/submission-1', {
      headers: expect.any(Headers),
    })
    expect(onFallback).toHaveBeenCalledWith(
      'Koneksi real-time terputus. Status diperbarui melalui polling setiap 3 detik.',
    )
    expect(onStatus).toHaveBeenCalledWith({
      status: 'running',
      current_test: 1,
      total_tests: 3,
    })
    cleanup()
  })

  it('stops polling status updates after a terminal status', async () => {
    const onStatus = vi.fn()
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        id: 'submission-1',
        status: 'completed',
        current_test: 3,
        total_tests: 3,
      }),
    })
    vi.stubGlobal('fetch', fetchMock)
    const cleanup = subscribeToSubmissionStatus('submission-1', 'jwt-value', {
      onStatus,
      onFallback: vi.fn(),
      onError: vi.fn(),
    })

    sockets[0].onclose()

    await vi.waitFor(() => {
      expect(onStatus).toHaveBeenCalledWith({
        status: 'completed',
        current_test: 3,
        total_tests: 3,
      })
    })
    cleanup()
    expect(sockets[0].close).toHaveBeenCalled()
  })

  it('simulates queued, running, and completed statuses in mock mode', () => {
    vi.stubEnv('VITE_USE_MOCK', 'true')
    vi.useFakeTimers()
    const onStatus = vi.fn()
    const cleanup = subscribeToSubmissionStatus('submission-1', 'jwt-value', {
      onStatus,
      onFallback: vi.fn(),
      onError: vi.fn(),
    })

    expect(onStatus).toHaveBeenNthCalledWith(1, {
      status: 'queued',
      current_test: 0,
      total_tests: 3,
    })
    vi.advanceTimersByTime(1800)
    expect(onStatus).toHaveBeenCalledWith({
      status: 'running',
      current_test: 2,
      total_tests: 3,
    })
    vi.advanceTimersByTime(600)
    expect(onStatus).toHaveBeenLastCalledWith({
      status: 'completed',
      current_test: 3,
      total_tests: 3,
    })
    cleanup()
    vi.useRealTimers()
  })
})
