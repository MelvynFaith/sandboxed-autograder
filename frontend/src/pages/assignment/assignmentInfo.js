import { useEffect, useState } from 'react'

// The two deadlines of FR-04: before deadline_at a submission is on time, until late_until_at it is
// accepted and marked late, after that it is refused. The server decides; this only labels it.
export function submissionWindow(assignment, now) {
  if (now <= Date.parse(assignment.deadlineAt)) {
    return 'open'
  }
  if (now <= Date.parse(assignment.lateUntilAt)) {
    return 'late'
  }
  return 'closed'
}

export const windowLabels = {
  open: 'Dibuka',
  late: 'Masa terlambat',
  closed: 'Ditutup',
}

export const windowStyles = {
  open: 'bg-success-tint text-success',
  late: 'bg-attention-tint text-attention',
  closed: 'bg-raised text-ink',
}

// The current time, refreshed every intervalMs, so a page left open moves from "open" to "late"
// to "closed" without a reload.
export function useNow(intervalMs = 30000) {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), intervalMs)
    return () => window.clearInterval(timer)
  }, [intervalMs])
  return now
}

function plural(value, unit) {
  return `${value} ${unit}`
}

export function formatMilliseconds(ms) {
  return ms % 1000 === 0 ? plural(ms / 1000, 'detik') : plural(ms, 'ms')
}

export function formatBytes(bytes) {
  if (bytes >= 1024 * 1024 && bytes % (1024 * 1024) === 0) {
    return `${bytes / (1024 * 1024)} MB`
  }
  if (bytes >= 1024 && bytes % 1024 === 0) {
    return `${bytes / 1024} KB`
  }
  return `${bytes} byte`
}

// The limits a student needs to know, in the order Codeforces shows them: time, then memory.
export function describeLimits(limits) {
  return [
    { label: 'Batas waktu CPU per test', value: formatMilliseconds(limits.cpu_time_ms) },
    { label: 'Batas waktu total per test', value: plural(limits.timeout_s, 'detik') },
    { label: 'Batas memori', value: `${limits.memory_mb} MB` },
    { label: 'Batas keluaran', value: formatBytes(limits.max_output_bytes) },
  ]
}
