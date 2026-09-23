# Infrastructure

Provisioning and deployment for the two environments. Staging and production are installed from
the same scripts and differ only in their `.env` file, database, and domain.

| Environment | Host | Deployed from |
|---|---|---|
| Staging | Free-tier cloud VM | merges to `develop` |
| Production | Campus server, reachable from outside the campus network | merges to `main` |

**Status:** not started.

## Planned contents

- Dependency installation and an unprivileged service user (`grader`).
- A systemd unit for the backend. The worker unit **must** set `Delegate=yes` so the service user
  owns its cgroup subtree.
- An AppArmor profile for `sandbox-exec`, or a host-level sysctl change, where the distribution
  restricts unprivileged user namespaces.
- An Nginx reverse proxy with TLS that serves the frontend build and uses `/healthz` as a health
  check.
- Database migrations, a seed command for the first admin account, a daily `pg_dump` with 14-day
  retention, and expiry of submission files.

Secrets are never stored here. They live in GitHub Actions secrets, separately for each environment.
