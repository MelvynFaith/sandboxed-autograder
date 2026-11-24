// Display labels and styles shared by the history, report, recap and code views.

export const submissionStatusLabels = {
  QUEUED: 'Antre',
  RUNNING: 'Sedang berjalan',
  COMPLETED: 'Selesai',
  FAILED: 'Pipeline gagal',
  CANCELLED: 'Dibatalkan',
}

export const submissionStatusStyles = {
  QUEUED: 'bg-attention-tint text-attention',
  RUNNING: 'bg-accent-tint text-link',
  COMPLETED: 'bg-success-tint text-success',
  FAILED: 'bg-danger-tint text-danger',
  CANCELLED: 'bg-raised text-ink',
}

export const verdictStyles = {
  PASSED: 'bg-success-tint text-success',
  FAILED: 'bg-danger-tint text-danger',
  TIMEOUT: 'bg-attention-tint text-attention',
  MEMORY_LIMIT_EXCEEDED: 'bg-done-tint text-done',
  RUNTIME_ERROR: 'bg-danger-tint text-danger',
  SYNTAX_ERROR: 'bg-danger-tint text-danger',
  SECURITY_VIOLATION: 'bg-ink text-white',
}

// Timestamps are stored in UTC and shown in WIB (UTC+7).
export const dateFormatter = new Intl.DateTimeFormat('id-ID', {
  dateStyle: 'long',
  timeStyle: 'short',
  timeZone: 'Asia/Jakarta',
})

// Shows "total / max" once a submission is graded. Both scores are omitted by the API until then
// (and while it is QUEUED, RUNNING or CANCELLED), which is not an error.
export function formatScore(totalScore, maxScore) {
  if (totalScore == null) {
    return 'Belum dinilai'
  }
  return maxScore == null ? String(totalScore) : `${totalScore} / ${maxScore}`
}
