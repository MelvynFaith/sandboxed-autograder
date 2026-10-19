// SPDX-License-Identifier: MIT
//
// Rate-limit test of SRS Bagian 11.4 (FR-14; infra/loadtest/README.md). Account 1 sends SPAM
// submissions in a row while OTHERS accounts submit one each. FR-14 holds one submission per
// student and assignment in the queue and wants 30 s before the next, so exactly one of the SPAM
// may be accepted, and every other must be refused with 429 and an error message saying why. The
// other accounts must still be graded, within NFR-01a's 10 s on average.
//
//   export LOAD_PASSWORD=...
//   k6 run infra/loadtest/ratelimit.js
//
// Options (-e): SPAM (200), OTHERS (10), and API and WAIT_S as in lib.js. Run `seed.sh reset`
// before it if the accounts submitted in the last 30 s.
import exec from 'k6/execution'
import { Counter, Rate, Trend } from 'k6/metrics'
import {
  assignments, email, login, submit, submitAndFollow, wholeNumber, WAIT_S,
} from './lib.js'

const SPAM = wholeNumber('SPAM', 200, 2, 10000)
const OTHERS = wholeNumber('OTHERS', 10, 1, 998)

const accepted = new Counter('spam_accepted')
const refused = new Rate('spam_refused_with_reason')
const e2e = new Trend('e2e_duration', true)
const completed = new Rate('completed')

export const options = {
  scenarios: {
    spam: {
      executor: 'per-vu-iterations',
      exec: 'spam',
      vus: 1,
      iterations: 1,
      maxDuration: `${WAIT_S + 60}s`,
    },
    others: {
      executor: 'per-vu-iterations',
      exec: 'others',
      vus: OTHERS,
      iterations: 1,
      maxDuration: `${WAIT_S + 60}s`,
    },
  },
  setupTimeout: `${30 + OTHERS}s`,
  thresholds: {
    spam_accepted: ['count==1'],
    spam_refused_with_reason: ['rate==1'],
    completed: ['rate==1'],
    e2e_duration: ['avg<=10000'],
  },
  summaryTrendStats: ['avg', 'min', 'med', 'p(95)', 'max'],
}

export function setup() {
  const tokens = login(1, 1 + OTHERS)
  return { tokens, assignment: assignments(tokens[1]).main }
}

export function spam(data) {
  const reasons = new Map()
  accepted.add(0)
  for (let i = 0; i < SPAM; i++) {
    const res = submit(data.tokens[1], data.assignment, 'spam')
    if (res.status === 202) {
      accepted.add(1)
      continue
    }
    let error
    try {
      error = res.json('error')
    } catch {
      error = undefined
    }
    refused.add(res.status === 429 && Boolean(error && error.code && error.message))
    const reason = `HTTP ${res.status} ${error ? `${error.code}: ${error.message}` : res.body}`
    reasons.set(reason, (reasons.get(reason) || 0) + 1)
  }
  for (const [reason, n] of reasons) {
    console.log(`${email(1)}: ${n} of ${SPAM} refused with ${reason}`)
  }
}

export function others(data) {
  const account = 2 + exec.scenario.iterationInTest
  submitAndFollow(email(account), data.tokens[account], data.assignment, (final, ms) => {
    completed.add(final === 'COMPLETED')
    if (ms !== null) {
      e2e.add(ms)
    }
  })
}
