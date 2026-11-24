import { describe, expect, it } from 'vitest'
import { describeLimits, formatBytes, formatMilliseconds, submissionWindow } from './assignmentInfo.js'

const assignment = {
  deadlineAt: '2026-10-12T16:59:00Z',
  lateUntilAt: '2026-10-13T16:59:00Z',
}

describe('submissionWindow', () => {
  it.each([
    ['2026-10-12T16:58:59Z', 'open'],
    ['2026-10-12T16:59:00Z', 'open'],
    ['2026-10-12T16:59:01Z', 'late'],
    ['2026-10-13T16:59:00Z', 'late'],
    ['2026-10-13T16:59:01Z', 'closed'],
  ])('at %s is %s', (now, expected) => {
    expect(submissionWindow(assignment, Date.parse(now))).toBe(expected)
  })
})

describe('limit formatting', () => {
  it.each([
    [1000, '1 detik'],
    [2500, '2500 ms'],
  ])('%d ms reads %s', (ms, text) => {
    expect(formatMilliseconds(ms)).toBe(text)
  })

  it.each([
    [1048576, '1 MB'],
    [65536, '64 KB'],
    [1000, '1000 byte'],
  ])('%d bytes reads %s', (bytes, text) => {
    expect(formatBytes(bytes)).toBe(text)
  })

  it('lists time, then memory, then output', () => {
    expect(
      describeLimits({ cpu_time_ms: 1000, timeout_s: 5, memory_mb: 128, max_output_bytes: 65536 }),
    ).toEqual([
      { label: 'Batas waktu CPU per test', value: '1 detik' },
      { label: 'Batas waktu total per test', value: '5 detik' },
      { label: 'Batas memori', value: '128 MB' },
      { label: 'Batas keluaran', value: '64 KB' },
    ])
  })
})
