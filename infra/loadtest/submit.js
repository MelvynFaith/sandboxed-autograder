// SPDX-License-Identifier: MIT
//
// End-to-end load test (SRS Bagian 11.4; infra/loadtest/README.md). ACCOUNTS students, seeded by
// seed.sh, each submit one program at the same moment and wait on the WebSocket for its final
// status. The measured time runs from the start of POST /api/v1/submissions to the final frame,
// not to the HTTP response, which only says the submission is queued.
//
//   export LOAD_PASSWORD=...
//   k6 run -e ACCOUNTS=30 -e ASSIGNMENT=ID infra/loadtest/submit.js
//
// Options (-e): ACCOUNTS (30), ASSIGNMENT (required), API (http://127.0.0.1:8080), WAIT_S (300),
// the longest a submission may take. LOAD_PASSWORD is read from the environment so that it stays
// off the command line.
//
// Thresholds: every submission must end COMPLETED (NFR-01b: none lost, every one processed to the
// end), and up to 30 accounts, the typical load of NFR-01a, the average must be at most 10 s. Above
// 30, the stress ceiling, the latency is a baseline to report, not a target. The scores are
// checked in the database afterwards with `seed.sh report`.
import http from 'k6/http'
import { check } from 'k6'
import { Rate, Trend } from 'k6/metrics'
import { WebSocket } from 'k6/websockets'

const API = __ENV.API || 'http://127.0.0.1:8080'
const ACCOUNTS = Number(__ENV.ACCOUNTS || 30)
const ASSIGNMENT = Number(__ENV.ASSIGNMENT)
const WAIT_S = Number(__ENV.WAIT_S || 300)
const PASSWORD = __ENV.LOAD_PASSWORD

if (!Number.isInteger(ACCOUNTS) || ACCOUNTS < 1 || ACCOUNTS > 999) {
  throw new Error(`ACCOUNTS must be a whole number from 1 to 999, not '${__ENV.ACCOUNTS}'`)
}
if (!Number.isInteger(ASSIGNMENT) || ASSIGNMENT < 1) {
  throw new Error('set -e ASSIGNMENT to the id that seed.sh printed')
}
if (!Number.isFinite(WAIT_S) || WAIT_S <= 0) {
  throw new Error(`WAIT_S must be a positive number of seconds, not '${__ENV.WAIT_S}'`)
}
if (!PASSWORD) {
  throw new Error('export LOAD_PASSWORD, the password seed.sh gave the students')
}

// Doubles its input, which passes all five test cases seed.sh creates.
const PROGRAM = 'print(int(input()) * 2)\n'
const TERMINAL = new Set(['COMPLETED', 'FAILED', 'CANCELLED'])
const JSON_HEADERS = { 'Content-Type': 'application/json' }

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
  // setup logs every account in, one after another: Argon2id makes each login take a while.
  setupTimeout: `${30 + ACCOUNTS}s`,
  thresholds: {
    completed: ['rate==1'],
    ...(ACCOUNTS <= 30 ? { e2e_duration: ['avg<=10000'] } : {}),
  },
  summaryTrendStats: ['avg', 'min', 'med', 'p(95)', 'max'],
}

const email = (i) => `load-${String(i).padStart(3, '0')}@example.test`

export function setup() {
  const tokens = []
  for (let i = 1; i <= ACCOUNTS; i++) {
    const body = JSON.stringify({ email: email(i), password: PASSWORD })
    const res = http.post(`${API}/api/v1/auth/login`, body,
      { headers: JSON_HEADERS, tags: { name: 'login' } })
    if (res.status !== 200) {
      throw new Error(`logging in ${email(i)} failed with HTTP ${res.status}; ` +
        `was seed.sh run with at least ${ACCOUNTS} accounts and the same LOAD_PASSWORD?`)
    }
    tokens.push(res.json('access_token'))
  }
  return { tokens }
}

export default function (data) {
  const account = email(__VU)
  const auth = { Authorization: `Bearer ${data.tokens[__VU - 1]}` }
  const start = Date.now()
  const form = {
    assignment_id: String(ASSIGNMENT),
    file: http.file(PROGRAM, 'main.py', 'text/x-python'),
  }
  const res = http.post(`${API}/api/v1/submissions`, form,
    { headers: auth, tags: { name: 'submit' } })
  if (!check(res, { 'submission accepted (202)': (r) => r.status === 202 })) {
    console.error(`${account}: submission refused with HTTP ${res.status}: ${res.body}`)
    completed.add(false)
    return
  }
  const id = res.json('id')
  const ticket = http.post(`${API}/api/v1/ws-tickets`, JSON.stringify({ submission_id: id }),
    { headers: { ...auth, ...JSON_HEADERS }, tags: { name: 'ws-ticket' } })
  if (!check(ticket, { 'ticket issued (201)': (r) => r.status === 201 })) {
    console.error(`${account}: no WebSocket ticket for submission ${id}: HTTP ${ticket.status}`)
    completed.add(false)
    return
  }

  // A submission graded before the socket opens still gets its final frame, because the server
  // sends the current state first. That case is measured up to the frame's arrival, so a little
  // long.
  const url = `${API.replace(/^http/, 'ws')}/ws/v1/submissions/${id}` +
    `?ticket=${encodeURIComponent(ticket.json('ticket'))}`
  const ws = new WebSocket(url)
  let final = null
  const timer = setTimeout(() => ws.close(), WAIT_S * 1000)
  ws.onmessage = (event) => {
    const frame = JSON.parse(event.data)
    if (final === null && TERMINAL.has(frame.status)) {
      final = frame.status
      e2e.add(Date.now() - start)
      ws.close()
    }
  }
  ws.onerror = (event) => {
    console.error(`${account}: WebSocket error on submission ${id}: ${event.error}`)
  }
  ws.onclose = () => {
    clearTimeout(timer)
    completed.add(final === 'COMPLETED')
    if (final !== 'COMPLETED') {
      const ending = final || `without a final frame in ${WAIT_S} s`
      console.error(`${account}: submission ${id} ended ${ending}`)
    }
  }
}
