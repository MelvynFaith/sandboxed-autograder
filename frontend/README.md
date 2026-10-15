# Dashboard

React and TailwindCSS client for the secure sandboxed auto-grading platform.

**Owner:** Benedictus Erlangga

## Development

Install dependencies with `npm ci`. Start Vite with `npm run dev`; API calls are
proxied to `http://localhost:8080`. Set `VITE_USE_MOCK=true` to use the local
demo data. Demo accounts are `mahasiswa@example.test`, `dosen@example.test`, and
`admin@example.test`; each uses `password123`.

Run checks before opening a pull request:

```sh
npm run lint
npm test
npm run build
```

## Frontend scope

- Students can browse assignments, submit Python files, follow grading status,
  and view submission history and test-case reports.
- Lecturers can manage courses, assignments, test cases, and resource limits.
- Administrators can manage user roles and review audit logs.
- Grade summaries/CSV export (FR-11) are not included in this PR; ownership is
  pending a team decision.

The UI is expected to work at 360 px, 768 px, and 1280 px without horizontal
scrolling. Forms have keyboard support and accessible labels. Timestamps are
stored in UTC and displayed in WIB (UTC+7).

## Authentication and session limitations

Login stores the access token, refresh token, and user in browser storage.
Expired access tokens are refreshed through the API, and logout calls the API
before clearing the local session. Refresh responses may omit a replacement
refresh token; when they do, the current refresh token is retained. If refresh
fails, the local session is cleared and the user is redirected to login. Logout
also clears the local session when the server request fails, with a notice on
the login page. Routes are gated by role. The API uses
uppercase roles (`STUDENT`, `LECTURER`, `ADMIN`); the frontend maps those to its
Indonesian role names for display and routing.

Tokens in `localStorage` are exposed to scripts running in the page, including
scripts introduced by cross-site scripting. This is a project limitation;
production deployments should prefer appropriately protected cookie-based
sessions and strong XSS defenses.

## Asumsi kontrak API

The SRS is authoritative. These are implementation mappings or details that
the frontend currently relies on; confirm exact payloads with the backend in
[issue #9](https://github.com/MelvynFaith/sandboxed-autograder/issues/9).

| Area | Frontend contract |
| --- | --- |
| Authentication | `POST /api/v1/auth/login` returns `access_token`, `refresh_token`, and a user with `id`, `name`, `email`, and uppercase `role`. Refresh uses `POST /api/v1/auth/refresh`; logout uses `POST /api/v1/auth/logout`. |
| Course and assignments | Course-scoped assignment listing/creation uses `/api/v1/courses/{course_id}/assignments`. Assignment fields use the SRS names `title`, `description`, `deadline_at`, and `late_until_at`. Assignment edit/delete use `PUT`/`DELETE /api/v1/assignments/{id}`. |
| Resource limits | Read/update use `/api/v1/assignments/{id}/resource-limit`; request fields include `cpu_time_ms`, `memory_mb`, `max_processes`, `timeout_s`, `max_output_bytes`, and `tmpfs_size_mb`. |
| Test cases | List/add are scoped to `/api/v1/assignments/{id}/testcases`; edit/delete use `/api/v1/testcases/{id}`. Fields use `external_id`, `input`, `expected_output`, `weight`, `is_hidden`, and `match_mode` (`EXACT` or `WHITESPACE_INSENSITIVE`). |
| Upload | Submission uses `POST /api/v1/submissions` with `assignment_id` and a `.py` file. The client rejects files larger than 256 KiB; the backend must enforce the limit as well. |
| Status and WebSocket | The frontend obtains a one-time ticket from `POST /api/v1/ws-tickets` with `{ "submission_id" }`, then connects to `WS /ws/v1/submissions/{id}?ticket=<ticket>`. Status values are `QUEUED`, `RUNNING`, `COMPLETED`, `FAILED`, or `CANCELLED`; the UI maps them for display. On disconnect it polls `GET /api/v1/submissions/{id}` every 3 seconds. |
| History and report | History uses `GET /api/v1/submissions`; report uses `GET /api/v1/submissions/{id}/report`. History/report fields follow the SRS English names, including `assignment_title`, `total_score`, `score`, `exec_time_ms`, and `memory_kb`. Per-test verdicts use the seven SRS enum values. |
| Admin | `GET /api/v1/admin/users`, `PUT /api/v1/admin/users/{id}/role`, and `GET /api/v1/admin/audit-logs` support the role-management and audit-log view. The API adapters are in `src/api/admin.js`. |

The lecturer form validates the SRS resource constraints
`tmpfs_size_mb + 16 <= memory_mb` and
`2 * max_output_bytes < tmpfs_size_mb * 2^20` before saving limits through
`PUT /api/v1/assignments/{id}/resource-limit`. Resource values must be positive
integers, and `late_until_at` must be at or after `deadline_at`.

Assignment listing and admin/history lists currently request `page=1&page_size=50`
and accept either an array or an `{ "items": [...] }` response. The frontend maps
the SRS API field names to UI models in the API modules; those mappings do not
change the backend contract.

API failures read the SRS `{ "error": { "code", "message" } }` envelope and
display its message. HTTP 429 responses show a rate-limit notice and honor a
numeric `Retry-After` header when present.

The WebSocket ticket is placed in the connection URL because the browser
WebSocket API does not support custom Authorization headers. A one-time ticket
limits exposure compared with putting a reusable JWT in the URL, but URLs may
still appear in infrastructure logs. Avoid logging tickets and expire/consume
them server-side.
