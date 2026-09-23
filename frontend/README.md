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

To be added with the first implementation. CI activates the `frontend` job once
`package-lock.json` exists here. The job runs `npm ci`, then the `lint` and `test` scripts if they
exist, then `npm run build`.
