// Labels and formatters for the grading views (recap, report and code pages).

export const statusLabels = {
  QUEUED: 'Antre',
  RUNNING: 'Sedang berjalan',
  COMPLETED: 'Selesai',
  FAILED: 'Pipeline gagal',
  CANCELLED: 'Dibatalkan',
}

export const statusStyles = {
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
  SECURITY_VIOLATION: 'bg-ink text-canvas',
}

// Timestamps are stored in UTC and shown in WIB (UTC+7).
export const dateFormatter = new Intl.DateTimeFormat('id-ID', {
  dateStyle: 'medium',
  timeStyle: 'short',
  timeZone: 'Asia/Jakarta',
})

// The API omits both scores until a submission is graded, which is not an error.
export function formatScore(totalScore, maxScore) {
  if (totalScore == null) {
    return 'Belum dinilai'
  }
  return maxScore == null ? String(totalScore) : `${totalScore} / ${maxScore}`
}
