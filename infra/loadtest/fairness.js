// SPDX-License-Identifier: MIT
//
// Queue fairness test of SRS Bagian 11.4 (FR-13; infra/loadtest/README.md). Account 1 floods the
// queue, then OTHERS accounts submit one each, and their average time from POST to final frame
// may be at most twice BASELINE_MS, the average without the flood.
//
// FR-14 lets an account queue one submission per assignment, so the flood is one submission to
// each of seed.sh's flood assignments, sent together at the end of setup, just before the others
// submit. The queue serves students in turn (FR-13), so the flood gets one turn in each round
// instead of making the others wait for all of it.
//
//   export LOAD_PASSWORD=...
//   k6 run -e ACCOUNTS=10 infra/loadtest/submit.js      # its e2e_duration avg is the baseline
//   infra/loadtest/seed.sh reset
//   k6 run -e BASELINE_MS=... infra/loadtest/fairness.js
//
// Options (-e): BASELINE_MS (required, in milliseconds), OTHERS (10), and API and WAIT_S as in
// lib.js.
import exec from 'k6/execution'
import { Rate, Trend } from 'k6/metrics'
import {
  assignments, email, login, submitAndFollow, submitEach, wholeNumber, WAIT_S,
} from './lib.js'

if (!('BASELINE_MS' in __ENV)) {
  throw new Error('set -e BASELINE_MS to the e2e_duration average, in ms, of submit.js ' +
    'with as many accounts as OTHERS')
}
const BASELINE_MS = wholeNumber('BASELINE_MS', 0, 1, 3600000)
const OTHERS = wholeNumber('OTHERS', 10, 1, 998)

const e2e = new Trend('e2e_duration', true)
const completed = new Rate('completed')

export const options = {
  scenarios: {
    others: {
      executor: 'per-vu-iterations',
      vus: OTHERS,
      iterations: 1,
      maxDuration: `${WAIT_S + 60}s`,
    },
  },
  setupTimeout: `${60 + OTHERS}s`,
  thresholds: {
    completed: ['rate==1'],
    e2e_duration: [`avg<=${2 * BASELINE_MS}`],
  },
  summaryTrendStats: ['avg', 'min', 'med', 'p(95)', 'max'],
}

export function setup() {
  const tokens = login(1, 1 + OTHERS)
  const { main, flood } = assignments(tokens[1])
  if (flood.length === 0) {
    throw new Error('course LOAD has no flood assignments; seed it again with FLOOD above 0')
  }
  const queued = submitEach(tokens[1], flood, 'flood').filter((r) => r.status === 202).length
  if (queued !== flood.length) {
    throw new Error(`only ${queued} of ${flood.length} flood submissions were accepted; ` +
      'run seed.sh reset first')
  }
  console.log(`${email(1)} queued ${queued} submissions, one per flood assignment`)
  return { tokens, assignment: main }
}

export default function (data) {
  const account = 2 + exec.scenario.iterationInTest
  submitAndFollow(email(account), data.tokens[account], data.assignment, (final, ms) => {
    completed.add(final === 'COMPLETED')
    if (ms !== null) {
      e2e.add(ms)
    }
  })
}
