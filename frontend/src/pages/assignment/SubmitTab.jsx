import { lazy, Suspense, useEffect, useId, useState } from 'react'
import { Link, useOutletContext } from 'react-router-dom'
import { MAX_FILE_SIZE, submitSubmission } from '../../api/submissions.js'
import SubmissionStatus from '../../components/SubmissionStatus.jsx'

// CodeMirror is the largest part of the bundle; it loads only when this view is opened.
const CodeEditor = lazy(() => import('../../components/CodeEditor.jsx'))

const EDITOR_FILENAME = 'main.py'

// Size in bytes as the server counts it: the UTF-8 encoding of the text.
function utf8Size(text) {
  return new TextEncoder().encode(text).length
}

function checkCode(code) {
  if (!code.trim()) {
    return 'Tulis kode Python sebelum mengirim.'
  }
  if (utf8Size(code) > MAX_FILE_SIZE) {
    return 'Ukuran kode maksimal 256 KB.'
  }
  return ''
}

function checkFile(file) {
  if (!file) {
    return 'Pilih berkas Python sebelum mengirim.'
  }
  if (!file.name.toLowerCase().endsWith('.py')) {
    return 'Pilih berkas dengan ekstensi .py.'
  }
  if (file.size > MAX_FILE_SIZE) {
    return 'Ukuran berkas maksimal 256 KB.'
  }
  return ''
}

// Seconds left until a rate-limited student may send again (FR-14), counted down once a second.
function useCountdown(until) {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    if (until <= Date.now()) {
      return undefined
    }
    const timer = window.setInterval(() => setNow(Date.now()), 1000)
    return () => window.clearInterval(timer)
  }, [until])
  return Math.max(0, Math.ceil((until - now) / 1000))
}

// Send code for this assignment, typed in the editor or chosen as a .py file. Both reach the API as
// a .py file upload (FR-04), so the server applies the same size, deadline and rate-limit checks.
function SubmitTab() {
  const { assignment, windowState } = useOutletContext()
  const languageId = useId()
  const fileInputId = useId()
  const [source, setSource] = useState('editor')
  const [code, setCode] = useState('')
  const [file, setFile] = useState(null)
  const [message, setMessage] = useState(null)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [submissionId, setSubmissionId] = useState(null)
  const [retryUntil, setRetryUntil] = useState(0)
  const secondsLeft = useCountdown(retryUntil)

  if (windowState === 'closed') {
    return (
      <p className="mt-6 rounded-lg border border-line bg-surface p-4 text-ink">
        Pengumpulan untuk assignment ini sudah ditutup.
      </p>
    )
  }

  async function handleSubmit(event) {
    event.preventDefault()
    const problem = source === 'editor' ? checkCode(code) : checkFile(file)
    if (problem) {
      setMessage({ type: 'failure', text: problem })
      return
    }

    const upload = source === 'editor'
      ? new File([code], EDITOR_FILENAME, { type: 'text/x-python' })
      : file
    setIsSubmitting(true)
    setMessage(null)
    try {
      const submission = await submitSubmission(assignment.id, upload)
      setSubmissionId(submission.id)
      setMessage({
        type: 'success',
        text: submission.isLate
          ? 'Kode terkirim dan ditandai terlambat.'
          : 'Kode terkirim dan sedang diantrekan untuk dinilai.',
      })
    } catch (error) {
      if (error.retryAfter) {
        setRetryUntil(Date.now() + error.retryAfter * 1000)
      }
      setMessage({ type: 'failure', text: error.message })
    } finally {
      setIsSubmitting(false)
    }
  }

  const waiting = secondsLeft > 0
  let buttonLabel = 'Kirim'
  if (isSubmitting) {
    buttonLabel = 'Mengirim...'
  } else if (waiting) {
    buttonLabel = `Kirim lagi dalam ${secondsLeft} detik`
  }

  return (
    <form className="mt-6 space-y-5" noValidate onSubmit={handleSubmit}>
      {windowState === 'late' && (
        <p className="rounded-md border border-attention/50 bg-attention-tint p-3 text-sm text-attention">
          Deadline sudah lewat. Kode yang dikirim sekarang tetap dinilai, tetapi ditandai terlambat.
        </p>
      )}
      <div>
        <label className="mb-1 block font-medium" htmlFor={languageId}>
          Bahasa
        </label>
        <select
          className="w-full max-w-xs rounded-md border border-line-strong bg-surface p-2"
          defaultValue="python3"
          id={languageId}
        >
          <option value="python3">Python 3 (standard library)</option>
        </select>
      </div>

      <fieldset>
        <legend className="mb-1 font-medium">Sumber kode</legend>
        <div className="flex flex-wrap gap-4">
          <label className="flex min-h-11 items-center gap-2">
            <input
              checked={source === 'editor'}
              name="source"
              onChange={() => setSource('editor')}
              type="radio"
              value="editor"
            />
            Tulis di editor
          </label>
          <label className="flex min-h-11 items-center gap-2">
            <input
              checked={source === 'file'}
              name="source"
              onChange={() => setSource('file')}
              type="radio"
              value="file"
            />
            Unggah berkas .py
          </label>
        </div>
      </fieldset>

      {source === 'editor' ? (
        <Suspense fallback={<p className="text-muted">Memuat editor...</p>}>
          <CodeEditor label="Kode Python" onChange={setCode} value={code} />
        </Suspense>
      ) : (
        <div>
          <label className="mb-1 block font-medium" htmlFor={fileInputId}>
            Berkas kode Python
          </label>
          <input
            accept=".py"
            className="block w-full min-w-0 rounded-md border border-line-strong bg-surface p-2"
            id={fileInputId}
            onChange={(event) => {
              setFile(event.target.files?.[0] ?? null)
              setMessage(null)
            }}
            type="file"
          />
          <p className="mt-1 text-sm text-muted">Berkas .py, maksimal 256 KB.</p>
        </div>
      )}

      {message && (
        <p
          className={message.type === 'success' ? 'text-sm text-link' : 'text-sm text-danger'}
          role={message.type === 'success' ? 'status' : 'alert'}
        >
          {message.text}
        </p>
      )}

      <div className="flex flex-wrap items-center gap-3">
        <button
          className="rounded-md bg-accent px-4 py-2 font-medium text-white hover:bg-accent-hover disabled:opacity-60"
          disabled={isSubmitting || waiting}
          type="submit"
        >
          {buttonLabel}
        </button>
        <Link className="text-link hover:underline" to="../submissions">
          Lihat submission saya
        </Link>
      </div>

      {submissionId && <SubmissionStatus key={submissionId} submissionId={submissionId} />}
    </form>
  )
}

export default SubmitTab
