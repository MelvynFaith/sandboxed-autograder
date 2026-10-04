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
requests are proxied to the local backend at `http://localhost:8080`.

Before opening a pull request, run:

```sh
npm run lint
npm test
npm run build
```
