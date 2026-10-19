// SPDX-License-Identifier: MIT
//
// Shared by the load-test scripts (infra/loadtest/README.md): their options, the accounts and
// assignments seed.sh made, and one submission followed on the WebSocket to its final frame.
//
// Every script reads -e API (http://127.0.0.1:8080) and -e WAIT_S (300), the longest a submission
// may take, and LOAD_PASSWORD from the environment, so that the password stays off the command
// line.
import http from 'k6/http'
import { check } from 'k6'
import { WebSocket } from 'k6/websockets'

export const API = __ENV.API || 'http://127.0.0.1:8080'
const PASSWORD = __ENV.LOAD_PASSWORD
if (!PASSWORD) {
  throw new Error('export LOAD_PASSWORD, the password seed.sh gave the students')
}

// wholeNumber reads the -e option name as a whole number from min to max, or fallback if unset.
export function wholeNumber(name, fallback, min, max) {
  const value = Number(name in __ENV ? __ENV[name] : fallback)
  if (!Number.isInteger(value) || value < min || value > max) {
    throw new Error(`${name} must be a whole number from ${min} to ${max}, not '${__ENV[name]}'`)
  }
  return value
}

export const WAIT_S = Number(__ENV.WAIT_S || 300)
if (!Number.isFinite(WAIT_S) || WAIT_S <= 0) {
  throw new Error(`WAIT_S must be a positive number of seconds, not '${__ENV.WAIT_S}'`)
}

// Doubles its input, which passes the five test cases of every assignment seed.sh makes.
const PROGRAM = 'print(int(input()) * 2)\n'
const TERMINAL = new Set(['COMPLETED', 'FAILED', 'CANCELLED'])
const JSON_HEADERS = { 'Content-Type': 'application/json' }

export const email = (i) => `load-${String(i).padStart(3, '0')}@example.test`

// login logs in the accounts first to last, one after another, and returns their access tokens
// keyed by account number. Argon2id makes each login take a while, so scripts do it in setup,
// outside the measurement.
export function login(first, last) {
  const tokens = {}
  for (let i = first; i <= last; i++) {
    const body = JSON.stringify({ email: email(i), password: PASSWORD })
    const res = http.post(`${API}/api/v1/auth/login`, body,
      { headers: JSON_HEADERS, tags: { name: 'login' } })
    if (res.status !== 200) {
      throw new Error(`logging in ${email(i)} failed with HTTP ${res.status}; ` +
        `was seed.sh run with at least ${last} accounts and the same LOAD_PASSWORD?`)
    }
    tokens[i] = res.json('access_token')
  }
  return tokens
}

function getList(url, token) {
  const res = http.get(url,
    { headers: { Authorization: `Bearer ${token}` }, tags: { name: 'list' } })
  if (res.status !== 200) {
    throw new Error(`GET ${url} failed with HTTP ${res.status}`)
  }
  const list = res.json()
  if (list.has_next) {
    throw new Error(`GET ${url} has more than one page`)
  }
  return list.items
}

// assignments finds seed.sh's course through the API, as the student with token sees it, and
// returns the id of the assignment every account submits to and those of the flood assignments.
export function assignments(token) {
  const course = getList(`${API}/api/v1/courses?page_size=100`, token)
    .find((c) => c.code === 'LOAD')
  if (!course) {
    throw new Error('the student is in no course LOAD; run seed.sh first')
  }
  const items = getList(`${API}/api/v1/courses/${course.id}/assignments?page_size=100`, token)
  const main = items.find((a) => a.title === 'Load test')
  if (!main) {
    throw new Error(`course LOAD (${course.id}) has no assignment 'Load test'`)
  }
  const flood = items.filter((a) => a.title.startsWith('Load test flood ')).map((a) => a.id)
  return { main: main.id, flood }
}

function submission(token, assignment, name) {
  return {
    method: 'POST',
    url: `${API}/api/v1/submissions`,
    body: {
      assignment_id: String(assignment),
      file: http.file(PROGRAM, 'main.py', 'text/x-python'),
    },
    params: { headers: { Authorization: `Bearer ${token}` }, tags: { name } },
  }
}

// submit uploads the program once; submitEach uploads it to each assignment at the same time.
export function submit(token, assignment, name) {
  const { url, body, params } = submission(token, assignment, name)
  return http.post(url, body, params)
}

export function submitEach(token, assignmentIDs, name) {
  return http.batch(assignmentIDs.map((id) => submission(token, id, name)))
}

// submitAndFollow submits for account and follows the submission on the WebSocket. onEnd is
// called exactly once, with the final status (null if none came) and the milliseconds from the
// start of the POST to the final frame (null without one).
export function submitAndFollow(account, token, assignment, onEnd) {
  const start = Date.now()
  const res = submit(token, assignment, 'submit')
  if (!check(res, { 'submission accepted (202)': (r) => r.status === 202 })) {
    console.error(`${account}: submission refused with HTTP ${res.status}: ${res.body}`)
    onEnd(null, null)
    return
  }
  const id = res.json('id')
  const ticket = http.post(`${API}/api/v1/ws-tickets`, JSON.stringify({ submission_id: id }),
    { headers: { Authorization: `Bearer ${token}`, ...JSON_HEADERS }, tags: { name: 'ws-ticket' } })
  if (!check(ticket, { 'ticket issued (201)': (r) => r.status === 201 })) {
    console.error(`${account}: no WebSocket ticket for submission ${id}: HTTP ${ticket.status}`)
    onEnd(null, null)
    return
  }

  // A submission graded before the socket opens still gets its final frame, because the server
  // sends the current state first. That case is measured up to the frame's arrival, so a little
  // long.
  const url = `${API.replace(/^http/, 'ws')}/ws/v1/submissions/${id}` +
    `?ticket=${encodeURIComponent(ticket.json('ticket'))}`
  const ws = new WebSocket(url)
  let final = null
  let elapsed = null
  const timer = setTimeout(() => ws.close(), WAIT_S * 1000)
  ws.onmessage = (event) => {
    const frame = JSON.parse(event.data)
    if (final === null && TERMINAL.has(frame.status)) {
      final = frame.status
      elapsed = Date.now() - start
      ws.close()
    }
  }
  ws.onerror = (event) => {
    console.error(`${account}: WebSocket error on submission ${id}: ${event.error}`)
  }
  ws.onclose = () => {
    clearTimeout(timer)
    if (final !== 'COMPLETED') {
      const ending = final || `without a final frame in ${WAIT_S} s`
      console.error(`${account}: submission ${id} ended ${ending}`)
    }
    onEnd(final, elapsed)
  }
}
