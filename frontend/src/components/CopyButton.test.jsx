import '@testing-library/jest-dom/vitest'
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import CopyButton from './CopyButton.jsx'
import { copyText } from './clipboard.js'

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  delete document.execCommand
})

describe('copyText', () => {
  it('uses the Clipboard API in a secure context', async () => {
    const writeText = vi.fn().mockResolvedValue()
    vi.stubGlobal('isSecureContext', true)
    vi.stubGlobal('navigator', { clipboard: { writeText } })

    await copyText('8\n')

    expect(writeText).toHaveBeenCalledWith('8\n')
  })

  it('falls back to a selected textarea over plain HTTP and removes it afterwards', async () => {
    vi.stubGlobal('isSecureContext', false)
    let copied
    document.execCommand = vi.fn(() => {
      copied = document.activeElement?.value ?? document.querySelector('textarea')?.value
      return true
    })

    await copyText('YES\n')

    expect(document.execCommand).toHaveBeenCalledWith('copy')
    expect(copied).toBe('YES\n')
    expect(document.querySelector('textarea')).toBeNull()
  })

  it('fails when the browser refuses to copy', async () => {
    vi.stubGlobal('isSecureContext', false)
    document.execCommand = vi.fn(() => false)

    await expect(copyText('x')).rejects.toThrow()
    expect(document.querySelector('textarea')).toBeNull()
  })
})

describe('CopyButton', () => {
  it('confirms the copy and returns to its label', async () => {
    vi.useFakeTimers()
    vi.stubGlobal('isSecureContext', false)
    document.execCommand = vi.fn(() => true)
    render(<CopyButton label="Salin input contoh 1" text="8" />)

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: 'Salin input contoh 1' }))
    })
    expect(screen.getByRole('button')).toHaveTextContent('Tersalin')
    expect(screen.getByText('Tersalin ke clipboard.')).toBeInTheDocument()

    act(() => vi.advanceTimersByTime(2000))
    expect(screen.getByRole('button')).toHaveTextContent('Salin')
    vi.useRealTimers()
  })

  it('tells a screen reader when copying failed', async () => {
    vi.stubGlobal('isSecureContext', false)
    document.execCommand = vi.fn(() => false)
    render(<CopyButton label="Salin output contoh 1" text="YES" />)

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: 'Salin output contoh 1' }))
    })

    expect(screen.getByText(/Gagal menyalin/)).toBeInTheDocument()
  })
})
