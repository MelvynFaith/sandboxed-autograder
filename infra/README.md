# Infrastructure

Provisioning and deployment for the two environments. Staging and production are installed from
the same scripts and differ only in their `.env` file, database, and domain.

| Environment | Host | Deployed from |
|---|---|---|
| Staging | Free-tier cloud VM | merges to `develop` |
| Production | Campus server, reachable from outside the campus network | merges to `main` |

**Status:** the AppArmor profile and the systemd unit exist. Everything under
[Not started](#not-started) does not.

## Contents

| Path | What it is | SRS |
|---|---|---|
| `apparmor/sandbox-exec` | AppArmor profile that lets `sandbox-exec` create user namespaces | R9 |
| `systemd/autograder.service` | Backend unit; delegates a cgroup subtree to the service user | R11, Bagian 13.2 |

## Sandbox host setup

Run these on the target host, from a checkout of this repository.

1. **Packages.** `sudo apt install build-essential libseccomp-dev git` (libseccomp 2.5 or newer).
2. **Service user.** `sudo useradd --system --no-create-home --shell /usr/sbin/nologin grader`
3. **Build and install `sandbox-exec`.** `make -C sandbox && sudo make -C sandbox install`. This
   puts the binary at `/usr/local/bin/sandbox-exec`, the path the profile below names.
4. **AppArmor profile, if the host needs it.** Ubuntu 24.04 and later restrict unprivileged user
   namespaces. Check with `sysctl kernel.apparmor_restrict_unprivileged_userns`: if it prints `1`,
   without a profile every `mount` inside the sandbox fails with `EPERM`.

   ```sh
   sudo install -m 0644 infra/apparmor/sandbox-exec /etc/apparmor.d/sandbox-exec
   sudo apparmor_parser -r /etc/apparmor.d/sandbox-exec
   sudo aa-status | grep sandbox-exec
   ```

   The last command must print `sandbox-exec`. The profile attaches by path, so always run the
   installed binary: a copy anywhere else, including `sandbox/sandbox-exec`, stays restricted. The
   profile is unconfined apart from `userns,`; it takes no permission away from the binary. The
   `abi <abi/4.0>` line needs AppArmor 4 (Ubuntu 24.04 or later). Hosts where the sysctl is absent
   or `0` do not need the profile. Setting the sysctl to `0` instead removes the restriction for
   every user of the host, so do not do that on a shared server.
5. **Unit.**

   ```sh
   sudo install -m 0644 infra/systemd/autograder.service /etc/systemd/system/autograder.service
   sudo install -d -m 0755 /etc/autograder
   sudo systemctl daemon-reload
   ```

   Put the environment's `.env` at `/etc/autograder/autograder.env`, owned by root with mode
   `0600`: systemd reads it before the service starts, so the service user needs no access. It
   never goes into the repository. `ExecStart` names `/usr/local/bin/autograder`, which does
   not exist yet; change it when the scheduler build lands, then `sudo systemctl enable --now
   autograder`.

## Verifying the host

Delegation works differently for a system service than for the user scope that `sandbox/tests/run.sh`
uses by default, so run the adversarial suite once as a transient system service with the same
`Delegate=yes`. Use a non-root account that can read the checkout:

```sh
sudo systemd-run --wait --pipe --collect -p User="$USER" -p Delegate=yes \
    "$PWD/sandbox/tests/run.sh" /usr/local/bin/sandbox-exec
```

Expected: `Ran 57 tests ... OK`. `run.sh` creates the leaf cgroup, moves itself into it and enables
the `memory`, `pids` and `cpu` controllers, which is what the worker has to do at start-up
(`sandbox/README.md`, "cgroup delegation"). Rerun this after any change to the unit's hardening
options.

## Not started

- Dependency installation as a script, and Go, Python, PostgreSQL and Node.js on the host.
- An Nginx reverse proxy with TLS that serves the frontend build and uses `/healthz` as a health
  check. The unit has no watchdog until the backend reports readiness.
- Database migrations, a seed command for the first admin account, a daily `pg_dump` with 14-day
  retention, and expiry of submission files.

Secrets are never stored here. They live in GitHub Actions secrets, separately for each environment.
