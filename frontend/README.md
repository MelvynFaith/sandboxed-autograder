# Dashboard

A React and TailwindCSS web client for students, lecturers, and administrators.

**Owner:** Benedictus Erlangga
**Status:** not started. Sprint 2 target: integration with the API and real-time status.

## Scope

- **Students:** upload code, follow grading progress live, browse submission history and
  per-test-case reports.
- **Lecturers:** manage courses, enrollment, assignments, test cases, and resource limits; view and
  export grade reports; read submitted code; trigger regrades.
- **Administrators:** manage user roles, the audit log, queue status, and rate limits.

Requirements: usable at 360 px, 768 px, and ≥ 1280 px widths without horizontal scroll; every form
works with the keyboard and has screen-reader labels. Timestamps are stored in UTC and shown in WIB
(UTC+7).

## Build and test

Install dependencies with `npm ci`. Start the Vite development server with `npm run dev`; `/api`
requests and `/ws` WebSocket connections are proxied to the local backend at
`http://localhost:8080`.

Set `VITE_USE_MOCK=true` when starting Vite to use the mock login. The demo accounts are
`mahasiswa@example.test`, `dosen@example.test`, and `admin@example.test`; each uses the password
`password123`. The API response mapping assumption is kept in `src/api/auth.js`.

The login page stores the JWT and user in the authentication context and browser storage. Routes for
assignments and history require an authenticated user; submission history is available to students
only.

The JWT is stored in `localStorage`, which is susceptible to theft through cross-site scripting
(XSS). This is an accepted limitation for this project; production deployments should consider
safer cookie-based session storage and protect against XSS.

## Asumsi kontrak API (belum disepakati dengan backend)

| Endpoint | Asumsi sementara |
| --- | --- |
| `POST /api/v1/auth/login` | Request `{ "email", "password" }`; response `{ "token", "user": { "id", "nama", "email", "role" } }`. |
| `GET /api/v1/assignments` | Response array `[{ "id", "judul", "deskripsi", "deadline" }]`. |
| `POST /api/v1/submissions` | `multipart/form-data` dengan field `assignment_id` dan `file` (`.py`). |
| Respons submission | `{ "id", "assignment_id", "status": "queued", "submitted_at" }`. |
| Status melalui WebSocket | `WS /ws/v1/submissions/{id}?token=<JWT>`; pesan `{ "status", "current_test", "total_tests" }`, dengan status `queued`, `running`, `completed`, `error`, atau `timeout`. |
| Fallback status | Polling `GET /api/v1/submissions/{id}` setiap 3 detik; respons `{ "id", "status", "current_test", "total_tests" }`. Server menutup WebSocket setelah status terminal. |
| Riwayat submission | `GET /api/v1/submissions` -> `[{ "id", "assignment_id", "assignment_judul", "status", "skor_total", "submitted_at" }]`; asumsi ini ditambahkan karena SRS tidak menetapkan endpoint daftar riwayat. Hanya submission milik user login yang dikembalikan (backend membatasi berdasarkan JWT). |
| Laporan test case | `GET /api/v1/submissions/{id}/report` -> `{ "submission_id", "skor_total", "results": [{ "testcase_id", "status", "skor", "waktu_eksekusi_ms", "memori_kb", "pesan_error", "is_hidden", "actual_output" }] }`. `skor_total` dan `pesan_error` tidak ada di entitas SRS dan merupakan asumsi tambahan. |

Asumsi assignment diisolasi di `src/api/assignments.js`; asumsi unggah dan respons submission ada di
`src/api/submissions.js`; asumsi status WebSocket dan polling ada di `src/api/submissionStatus.js`.
Asumsi daftar riwayat dan laporan masing-masing diisolasi di `src/api/submissionHistory.js` dan
`src/api/submissionReport.js`. Laporan tidak mengembalikan output atau pesan error untuk test case
tersembunyi ke komponen UI.
Diskusi konfirmasi backend: [issue #9](https://github.com/MelvynFaith/sandboxed-autograder/issues/9).
Validasi frontend membatasi berkas Python hingga 1 MiB; backend tetap perlu memvalidasi ulang.

Browser WebSocket tidak dapat mengirim header `Authorization`, jadi token sementara dikirim sebagai
parameter query `token`. Token pada query dapat muncul di log server dan riwayat URL; ini merupakan
keterbatasan keamanan. Tiket WebSocket sekali pakai merupakan opsi yang lebih aman untuk dibahas
dengan backend sebagai pekerjaan lanjutan.

Before opening a pull request, run:

```sh
npm run lint
npm test
npm run build
```
