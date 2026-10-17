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
- Grade-summary and CSV-export screens (FR-11) remain to be implemented as
  part of the frontend dashboard scope. Confirm the backend endpoint contract
  and backend delivery owner with the team.

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
| Registration and profile | FR-01/FR-15 define `POST /api/v1/auth/register`, `GET /api/v1/users/me`, `PUT /api/v1/users/me` (name), and `PUT /api/v1/users/me/password`. The SRS does not define the registration request body or the password-change body field names; the frontend assumes `{ name, email, password }` for registration and `{ current_password, new_password }` for password change. Registration request construction is isolated to the register function in `src/api/auth.js`; profile/password request adapters are in `src/api/profile.js`. Registration always creates a `STUDENT`; password changes clear the local session after success. |
| Course and assignments | Course-scoped assignment listing/creation uses `/api/v1/courses/{course_id}/assignments`. Assignment fields use the SRS names `title`, `description`, `deadline_at`, and `late_until_at`. Assignment edit/delete use `PUT`/`DELETE /api/v1/assignments/{id}`. |
| Resource limits | Bagian 6.3 menetapkan `GET`/`PUT /api/v1/assignments/{id}/resource-limit`; FR-06 menetapkan field `cpu_time_ms`, `memory_mb`, `max_processes`, `timeout_s`, `max_output_bytes`, dan `tmpfs_size_mb` beserta invarian kapasitas yang divalidasi form. |
| Test cases | Endpoint berada di bawah `/api/v1/assignments/{id}/testcases` (Bagian 6.3). Field entitas `external_id`, `input`, `expected_output`, `weight`, `is_hidden`, dan `match_mode` mengikuti Bagian 7.1. |
| Upload | FR-04 menetapkan `POST /api/v1/submissions` dengan `assignment_id` dan berkas `.py`; berkas di atas 256 KB (frontend mengukur 256 × 1024 byte) ditolak, submission dalam jendela terlambat ditandai `is_late`, dan submission setelah `late_until_at` ditolak (409). FR-14 membatasi satu submission `QUEUED`/`RUNNING` per assignment, jeda minimum 30 detik, dan 30 submission per assignment per mahasiswa. UI menampilkan `is_late` tanpa mengubah skor. |
| Status and WebSocket | Bagian 6.3 menetapkan endpoint submission dan tiket WebSocket. Status Submission (`QUEUED`, `RUNNING`, `COMPLETED`, `FAILED`, `CANCELLED`) mengikuti Bagian 7.1/FR-08; tujuh verdict `Result.status` tetap berbeda dari status Submission. Saat WebSocket putus, UI melakukan polling `GET /api/v1/submissions/{id}` tiap 3 detik. |
| History and report | Riwayat menggunakan `GET /api/v1/submissions` dan laporan `GET /api/v1/submissions/{id}/report` (Bagian 6.3); daftar meminta parameter `page` dan `page_size`. Field laporan mengikuti nama Inggris di Bagian 7.1 dan FR hasil: `assignment_title`, `total_score`, `score`, `exec_time_ms`, dan `memory_kb`. |
| Admin | `GET /api/v1/admin/users`, `PUT /api/v1/admin/users/{id}/role`, dan `GET /api/v1/admin/audit-logs` mendukung panel admin (Bagian 6.3); kedua daftar mengirim `page` dan `page_size`. Adapter berada di `src/api/admin.js`. |

Sumber resource limits adalah FR-06: `tmpfs_size_mb + 16 <= memory_mb` dan
`2 * max_output_bytes < tmpfs_size_mb * 2^20`, dengan `max_output_bytes` dalam
byte serta MB = `2^20` byte. Form mengirim integer positif menggunakan
`PUT /api/v1/assignments/{id}/resource-limit`. `late_until_at` harus sama dengan
atau setelah `deadline_at`.

Assignment, history, user, and audit-log list adapters accept either an array or
an `{ "items": [...] }` response; paginated history/admin requests include the
current `page` and `page_size=50`, with previous/next controls. The UI reads an
optional `has_next` boolean when present; otherwise it infers whether another
page may exist from the returned item count and requested page size. The frontend
maps SRS API field names to UI models in API modules; those mappings do not
change the backend contract.

API failures read the SRS `{ "error": { "code", "message" } }` envelope and
display its message. Upload HTTP 409, 413, and 429 responses show specific
deadline, file-size, and frequency-limit messages. HTTP 429 responses honor a
numeric `Retry-After` header when present (FR-04/FR-14).

The WebSocket ticket is placed in the connection URL because the browser
WebSocket API does not support custom Authorization headers. A one-time ticket
limits exposure compared with putting a reusable JWT in the URL, but URLs may
still appear in infrastructure logs. Avoid logging tickets and expire/consume
them server-side.
