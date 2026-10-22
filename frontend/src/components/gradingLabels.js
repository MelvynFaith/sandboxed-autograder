// Labels and formatters for the grading views (recap, report and code pages).

export const statusLabels = {
  QUEUED: 'Antre',
  RUNNING: 'Sedang berjalan',
  COMPLETED: 'Selesai',
  FAILED: 'Pipeline gagal',
  CANCELLED: 'Dibatalkan',
}

export const statusStyles = {
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
