import { useEffect, useState } from 'react'
import { copyText } from './clipboard.js'

function CopyButton({ label, text }) {
  const [state, setState] = useState('idle')

  useEffect(() => {
    if (state === 'idle') {
      return undefined
    }
    const timer = window.setTimeout(() => setState('idle'), 2000)
    return () => window.clearTimeout(timer)
  }, [state])

  async function handleClick() {
    try {
      await copyText(text)
      setState('copied')
    } catch {
      setState('failed')
    }
  }

  return (
    <>
      <button
        aria-label={label}
        className="rounded-md border border-line-strong bg-surface px-2 py-0.5 text-xs font-medium hover:bg-raised"
        onClick={handleClick}
        type="button"
      >
        {state === 'copied' ? 'Tersalin' : 'Salin'}
      </button>
      <span aria-live="polite" className="sr-only">
        {state === 'copied' ? 'Tersalin ke clipboard.' : ''}
        {state === 'failed' ? 'Gagal menyalin; pilih teksnya lalu salin manual.' : ''}
      </span>
    </>
  )
}

export default CopyButton
