// SPDX-License-Identifier: MIT
//
// End-to-end load test (SRS Bagian 11.4; infra/loadtest/README.md). ACCOUNTS students, seeded by
// seed.sh, each submit one program at the same moment and wait on the WebSocket for its final
// status. The measured time runs from the start of POST /api/v1/submissions to the final frame,
// not to the HTTP response, which only says the submission is queued.
//
//   export LOAD_PASSWORD=...
//   k6 run -e ACCOUNTS=30 infra/loadtest/submit.js
//
// Options (-e): ACCOUNTS (30), and API and WAIT_S as in lib.js.
//
// Thresholds: every submission must end COMPLETED (NFR-01b: none lost, every one processed to the
// end), and up to 30 accounts, the typical load of NFR-01a, the average must be at most 10 s. Above
// 30, the stress ceiling, the latency is a baseline to report, not a target. The scores are
// checked in the database afterwards with `seed.sh report`.
import { Rate, Trend } from 'k6/metrics'
import { assignments, email, login, submitAndFollow, wholeNumber, WAIT_S } from './lib.js'

const ACCOUNTS = wholeNumber('ACCOUNTS', 30, 1, 999)

const e2e = new Trend('e2e_duration', true)
const completed = new Rate('completed')

export const options = {
  scenarios: {
    submit: {
      executor: 'per-vu-iterations',
      vus: ACCOUNTS,
      iterations: 1,
      maxDuration: `${WAIT_S + 60}s`,
    },
  },
  setupTimeout: `${30 + ACCOUNTS}s`,
  thresholds: {
    completed: ['rate==1'],
    ...(ACCOUNTS <= 30 ? { e2e_duration: ['avg<=10000'] } : {}),
  },
  summaryTrendStats: ['avg', 'min', 'med', 'p(95)', 'max'],
}

export function setup() {
  const tokens = login(1, ACCOUNTS)
  return { tokens, assignment: assignments(tokens[1]).main }
}

export default function (data) {
  submitAndFollow(email(__VU), data.tokens[__VU], data.assignment, (final, ms) => {
    completed.add(final === 'COMPLETED')
    if (ms !== null) {
      e2e.add(ms)
    }
  })
}
