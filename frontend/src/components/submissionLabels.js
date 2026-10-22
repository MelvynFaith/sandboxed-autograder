// Display labels and styles shared by the history, report, recap and code views.

export const submissionStatusLabels = {
  QUEUED: 'Antre',
  RUNNING: 'Sedang berjalan',
  COMPLETED: 'Selesai',
  FAILED: 'Pipeline gagal',
  CANCELLED: 'Dibatalkan',
}

export const submissionStatusStyles = {
  QUEUED: 'bg-amber-100 text-amber-900',
  RUNNING: 'bg-blue-100 text-blue-900',
  COMPLETED: 'bg-green-100 text-green-900',
  FAILED: 'bg-red-100 text-red-900',
  CANCELLED: 'bg-slate-200 text-slate-900',
}

export const verdictStyles = {
  PASSED: 'bg-green-100 text-green-900',
  FAILED: 'bg-red-100 text-red-900',
  TIMEOUT: 'bg-amber-100 text-amber-900',
  MEMORY_LIMIT_EXCEEDED: 'bg-purple-100 text-purple-900',
  RUNTIME_ERROR: 'bg-orange-100 text-orange-900',
  SYNTAX_ERROR: 'bg-pink-100 text-pink-900',
  SECURITY_VIOLATION: 'bg-slate-900 text-white',
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
