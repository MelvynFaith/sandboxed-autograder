import { useEffect, useState } from 'react'
import { subscribeToSubmissionStatus } from '../api/submissionStatus.js'

const statusLabels = {
  queued: 'Antre',
  running: 'Sedang berjalan',
  completed: 'Selesai',
  error: 'Error',
  timeout: 'Waktu habis',
}

const statusStyles = {
  queued: 'bg-amber-100 text-amber-900',
  running: 'bg-blue-100 text-blue-900',
  completed: 'bg-green-100 text-green-900',
  error: 'bg-red-100 text-red-900',
  timeout: 'bg-red-100 text-red-900',
}

function SubmissionStatus({ submissionId, token }) {
  const [status, setStatus] = useState({
    status: 'queued',
    current_test: 0,
    total_tests: 0,
  })
  const [fallbackMessage, setFallbackMessage] = useState('')
  const [errorMessage, setErrorMessage] = useState('')

  useEffect(() => {
    return subscribeToSubmissionStatus(submissionId, token, {
      onStatus: (nextStatus) => {
        setStatus(nextStatus)
        setErrorMessage('')
      },
      onFallback: setFallbackMessage,
      onError: setErrorMessage,
    })
  }, [submissionId, token])

  const progress = status.total_tests
    ? Math.round((status.current_test / status.total_tests) * 100)
    : 0

  return (
    <section aria-label={`Status submission ${submissionId}`} className="mt-4 space-y-3">
      <div className="flex flex-wrap items-center gap-3">
        <span
          className={`rounded-full px-3 py-1 text-sm font-medium ${statusStyles[status.status]}`}
          data-testid="submission-status-badge"
        >
          {statusLabels[status.status]}
        </span>
        {status.total_tests > 0 && (
          <span className="text-sm text-slate-700">
            Test {status.current_test} dari {status.total_tests}
          </span>
        )}
      </div>
      <div
        aria-label="Progress pengujian"
        aria-valuemax={status.total_tests || 1}
        aria-valuemin={0}
        aria-valuenow={status.current_test}
        className="h-2 overflow-hidden rounded-full bg-slate-200"
        role="progressbar"
      >
        <div
          className="h-full rounded-full bg-blue-700 transition-all"
          style={{ width: `${progress}%` }}
        />
      </div>
      {fallbackMessage && (
        <p className="text-sm text-amber-800" role="status">
          {fallbackMessage}
        </p>
      )}
      {errorMessage && (
        <p className="text-sm text-red-700" role="alert">
          Gagal memperbarui status: {errorMessage}
        </p>
      )}
    </section>
  )
}

export default SubmissionStatus
