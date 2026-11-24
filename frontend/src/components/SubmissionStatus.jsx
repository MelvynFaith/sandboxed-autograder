import { useEffect, useState } from 'react'
import { subscribeToSubmissionStatus } from '../api/submissionStatus.js'

const statusLabels = {
  QUEUED: 'Antre',
  RUNNING: 'Sedang berjalan',
  COMPLETED: 'Selesai',
  FAILED: 'Pipeline gagal',
  CANCELLED: 'Dibatalkan',
}

const statusStyles = {
  QUEUED: 'bg-attention-tint text-attention',
  RUNNING: 'bg-accent-tint text-link',
  COMPLETED: 'bg-success-tint text-success',
  FAILED: 'bg-danger-tint text-danger',
  CANCELLED: 'bg-raised text-ink',
}

function SubmissionStatus({ submissionId }) {
  const [status, setStatus] = useState({
    status: 'QUEUED',
    current_test: 0,
    total_tests: 0,
  })
  const [fallbackMessage, setFallbackMessage] = useState('')
  const [errorMessage, setErrorMessage] = useState('')

  useEffect(() => {
    return subscribeToSubmissionStatus(submissionId, {
      onStatus: (nextStatus) => {
        setStatus(nextStatus)
        setErrorMessage('')
      },
      onFallback: setFallbackMessage,
      onError: setErrorMessage,
    })
  }, [submissionId])

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
          <span className="text-sm text-ink">
            Test {status.current_test} dari {status.total_tests}
          </span>
        )}
      </div>
      {status.total_tests !== null && (
        <div
          aria-label="Progress pengujian"
          aria-valuemax={status.total_tests || 1}
          aria-valuemin={0}
          aria-valuenow={status.current_test}
          className="h-2 overflow-hidden rounded-full bg-raised"
          role="progressbar"
        >
          <div
            className="h-full rounded-full bg-accent transition-all"
            style={{ width: `${progress}%` }}
          />
        </div>
      )}
      {fallbackMessage && (
        <p className="text-sm text-attention" role="status">
          {fallbackMessage}
        </p>
      )}
      {errorMessage && (
        <p className="text-sm text-danger" role="alert">
          Gagal memperbarui status: {errorMessage}
        </p>
      )}
    </section>
  )
}

export default SubmissionStatus
