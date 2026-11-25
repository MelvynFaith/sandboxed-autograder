export const resourceLimitFields = [
  { name: 'cpu_time_ms', label: 'Batas waktu CPU (ms)' },
  { name: 'memory_mb', label: 'Batas memori (MB)' },
  { name: 'max_processes', label: 'Jumlah proses maksimum' },
  { name: 'timeout_s', label: 'Waktu maksimum (detik)' },
  { name: 'max_output_bytes', label: 'Output maksimum (byte)' },
  { name: 'tmpfs_size_mb', label: 'Ukuran tmpfs (MB)' },
]

export const emptyLimits = Object.fromEntries(resourceLimitFields.map(({ name }) => [name, '']))
export const emptyAssignment = { title: '', description: '', deadline_at: '', late_until_at: '' }

// The value of a datetime-local input is local time without a zone.
export function toLocalDateTime(value) {
  if (!value) {
    return ''
  }
  const date = new Date(value)
  return new Date(date.getTime() - date.getTimezoneOffset() * 60_000)
    .toISOString()
    .slice(0, 16)
}

export function assignmentToForm(assignment) {
  return {
    title: assignment.title,
    description: assignment.description,
    deadline_at: toLocalDateTime(assignment.deadlineAt),
    late_until_at: toLocalDateTime(assignment.lateUntilAt),
  }
}

export function limitsToForm(limits) {
  return Object.fromEntries(resourceLimitFields.map(({ name }) => [name, String(limits[name])]))
}

export function validateResourceLimits(limits) {
  if (Object.values(limits).some(
    (value) => !Number.isInteger(value) || value <= 0 || value > 2_147_483_647,
  )) {
    return 'Semua batas sumber daya harus berupa bilangan bulat positif.'
  }
  if (limits.tmpfs_size_mb + 16 > limits.memory_mb) {
    return 'Ukuran tmpfs ditambah 16 MB tidak boleh melebihi batas memori.'
  }
  if (2 * limits.max_output_bytes >= limits.tmpfs_size_mb * 1024 ** 2) {
    return 'Dua kali batas output harus lebih kecil dari ukuran tmpfs dalam byte.'
  }
  return null
}
