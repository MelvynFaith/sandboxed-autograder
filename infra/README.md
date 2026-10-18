# Infrastructure

Provisioning and deployment for the two environments. Staging and production are installed from
the same scripts and differ only in their `.env` file, database, and domain.

| Environment | Host | Deployed from |
|---|---|---|
| Staging | Free-tier cloud VM | merges to `develop` |
| Production | Campus server, reachable from outside the campus network | merges to `main` |

**Status:** the AppArmor profile, the systemd unit and the backend install steps exist. Everything
under [Not started](#not-started) does not.

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
   never goes into the repository. The variables are listed [below](#backend-environment).
6. **Backend.** Build it anywhere with Go 1.27 or newer; with cgo off the binary is static, so a
   build from a newer distribution still runs on Ubuntu 24.04. Then install it at the path the
   unit's `ExecStart` names and create the directory for stored submissions:

   ```sh
   (cd scheduler && CGO_ENABLED=0 go build -o autograder ./cmd/autograder)
   sudo install -m 0755 scheduler/autograder /usr/local/bin/autograder
   sudo install -d -m 0700 -o grader -g grader /var/lib/autograder/submissions
   sudo systemctl enable --now autograder
   journalctl -u autograder -n 20
   ```

   The log is one JSON object per line. A good start shows `database schema is up to date`,
   `delegated cgroup ready`, `swept leftovers of dead sandbox runs` and `scheduler API listening`.

### Backend environment

| Variable | Required | Meaning |
|---|---|---|
| `DATABASE_URL` | yes | `postgres://` URL. The backend applies the migrations at start-up, so this user must be able to create tables in the database. |
| `JWT_SECRET` | yes | At least 32 random bytes; signs the access tokens. |
| `HTTP_ADDR` | yes | Listen address, for example `127.0.0.1:8080` behind Nginx. |
| `WORKERS` | no | Submissions graded at once, default 4 (SRS Bagian 5.1). `0` runs the API alone and grades nothing. |
| `CODE_DIR` | unless `WORKERS=0` | Absolute path of the stored submission files, for example `/var/lib/autograder/submissions`. |
| `HARNESS_COMMAND` | unless `WORKERS=0` | The Grading Harness program and its leading arguments, split on spaces, the program as an absolute path. The Harness's command-line entry point is not merged yet. |
| `SANDBOX_EXEC` | no | Path of `sandbox-exec`, default `/usr/local/bin/sandbox-exec`, the path the AppArmor profile names. |

The Harness gets none of these: its environment is only `SANDBOX_EXEC` and `SANDBOX_CGROUP_ROOT`,
which the backend sets.

## Verifying the host

Delegation works differently for a system service than for the user scope that `sandbox/tests/run.sh`
uses by default, so run the adversarial suite once as a transient system service with the same
`Delegate=yes`. Use a non-root account that can read the checkout:

```sh
sudo systemd-run --wait --pipe --collect -p User="$USER" -p Delegate=yes \
    "$PWD/sandbox/tests/run.sh" /usr/local/bin/sandbox-exec
```

Expected: `Ran N tests ... OK`, where N is the size of the suite (64 at the time of writing).
`run.sh` creates the leaf cgroup, moves itself into it and enables the `memory`, `pids` and `cpu`
controllers, which is what the backend does at start-up (`sandbox/README.md`, "cgroup delegation").
Rerun this after any change to the unit's hardening options.

`make test`, `make asan` and `make trace` fail on Ubuntu 24.04 when run from the build directory, with
`write /proc/self/setgroups: Permission denied`: the AppArmor profile (`infra/apparmor/sandbox-exec`)
attaches only to `/usr/local/bin/sandbox-exec`, so a binary at any other path is still subject to
`kernel.apparmor_restrict_unprivileged_userns` (SRS risk R9). To run another build on the host, install it
at that path, run the suite from there, and put the normal binary back:

```sh
sudo install -m 0755 sandbox/sandbox-exec-trace /usr/local/bin/sandbox-exec    # or sandbox-exec-asan
sandbox/tests/run.sh /usr/local/bin/sandbox-exec
sudo make -C sandbox install
```

Build the variant first with `make -C sandbox CFLAGS=-DSBX_FORCE_TRACE BUILD_DIR=build/trace BIN=sandbox-exec-trace`
or `make -C sandbox SANITIZE=1 BUILD_DIR=build/asan BIN=sandbox-exec-asan`.

## Not started

- Dependency installation as a script, and Go, Python, PostgreSQL and Node.js on the host.
- An Nginx reverse proxy with TLS that serves the frontend build and uses `/healthz` as a health
  check. The unit has no watchdog until the backend reports readiness.
- A seed command for the first admin account, a daily `pg_dump` with 14-day retention, and expiry
  of submission files.

Secrets are never stored here. They live in GitHub Actions secrets, separately for each environment.
