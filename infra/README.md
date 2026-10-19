# Infrastructure

Provisioning and deployment for the two environments. Staging and production are installed from
the same scripts and differ only in their `.env` file, database, and domain.

| Environment | Host | Deployed from |
|---|---|---|
| Staging | Free-tier cloud VM | merges to `develop` |
| Production | Campus server, reachable from outside the campus network | merges to `main` |

**Status:** the AppArmor profile, the systemd unit, the install steps, a smoke test and a daily
database backup exist, and the backend runs on the campus server from these steps. Everything under
[Not started](#not-started) does not.

## Contents

| Path | What it is | SRS |
|---|---|---|
| `apparmor/sandbox-exec` | AppArmor profile that lets `sandbox-exec` create user namespaces | R9 |
| `systemd/autograder.service` | Backend unit; delegates a cgroup subtree to the service user | R11, Bagian 13.2 |
| `smoke-test.sh` | Three submissions through the installed pipeline, checked against their scores | Bagian 13.2, NFR-09 |
| `db-backup.sh`, `systemd/autograder-backup.{service,timer}` | Daily `pg_dump` with 14-day retention | Bagian 13.4, NFR-13 |

## Host setup

Run these on the target host, from a checkout of this repository. They were last run in this order on
Ubuntu 24.04 (the campus server).

1. **Packages.** libseccomp 2.5 or newer; Ubuntu 24.04 packages PostgreSQL 16, the version CI tests
   against.

   ```sh
   sudo apt install build-essential libseccomp-dev git postgresql python3-venv
   ```
2. **Service user.** `sudo useradd --system --no-create-home --shell /usr/sbin/nologin grader`
3. **Go, for building only.** Any machine with Go 1.27 or newer can build the backend (step 9). To
   build on the host without installing Go system-wide:

   ```sh
   curl -fsSLo /tmp/go.tgz https://go.dev/dl/go1.27.2.linux-amd64.tar.gz
   mkdir ~/go-sdk && tar -C ~/go-sdk --strip-components=1 -xzf /tmp/go.tgz && rm /tmp/go.tgz
   ```
4. **Build and install `sandbox-exec`.** `make -C sandbox && sudo make -C sandbox install`. This
   puts the binary at `/usr/local/bin/sandbox-exec`, the path the profile below names.
5. **AppArmor profile, if the host needs it.** Ubuntu 24.04 and later restrict unprivileged user
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
   every user of the host, so do not do that on a shared server. Then [verify the
   host](#verifying-the-host).
6. **Database.** A PostgreSQL role with the service user's name, owning its database. Ubuntu's
   default `pg_hba.conf` lets a local OS user log in as the role of the same name over the Unix
   socket (peer authentication), so the service needs no database password at all, and owning the
   database lets it apply the migrations itself at start-up.

   ```sh
   sudo -u postgres createuser grader
   sudo -u postgres createdb -O grader autograder
   ```

   The matching `DATABASE_URL` is `postgres://grader@/autograder?host=/var/run/postgresql`.
7. **Grading Harness.** In a virtual environment of its own, which the backend starts as
   `python -m harness`:

   ```sh
   sudo python3 -m venv /opt/grader/venv
   sudo /opt/grader/venv/bin/pip install "$PWD/grader"
   (cd / && sudo -u grader env -i /opt/grader/venv/bin/python -m harness --help | head -1)
   ```

   The last command must print the usage line: it runs the Harness the way the backend does, as
   `grader` with an empty environment.
8. **Unit and environment.** The environment file is owned by root with mode `0600`: systemd reads
   it before the service starts, so the service user needs no access. It never goes into the
   repository. The JWT secret is generated on the host and never leaves it. The variables are listed
   [below](#backend-environment).

   ```sh
   sudo install -m 0644 infra/systemd/autograder.service /etc/systemd/system/autograder.service
   sudo install -d -m 0755 /etc/autograder
   sudo install -m 0600 /dev/null /etc/autograder/autograder.env
   sudo tee /etc/autograder/autograder.env > /dev/null <<EOF
   DATABASE_URL=postgres://grader@/autograder?host=/var/run/postgresql
   JWT_SECRET=$(openssl rand -base64 48)
   HTTP_ADDR=127.0.0.1:8080
   WORKERS=4
   CODE_DIR=/var/lib/autograder/submissions
   HARNESS_COMMAND=/opt/grader/venv/bin/python -m harness
   EOF
   sudo systemctl daemon-reload
   ```

   Paste the `tee` command whole: a terminal that splits the paste leaves the shell waiting for
   `EOF`. `HTTP_ADDR` on the loopback address keeps the backend off the network until Nginx and TLS
   front it.
9. **Backend.** With cgo off the binary is static, so a build from a newer distribution still runs
   on Ubuntu 24.04. Install it at the path the unit's `ExecStart` names, create the directory for
   stored submissions, and start it:

   ```sh
   (cd scheduler && CGO_ENABLED=0 ~/go-sdk/bin/go build -o autograder ./cmd/autograder)
   sudo install -m 0755 scheduler/autograder /usr/local/bin/autograder
   sudo install -d -m 0700 -o grader -g grader /var/lib/autograder/submissions
   sudo systemctl enable --now autograder
   sudo journalctl -u autograder -n 20 -o cat
   curl -s http://127.0.0.1:8080/healthz
   ```

   The log is one JSON object per line. A good start shows `database schema is up to date`,
   `delegated cgroup ready`, `grading worker pool started`, `scheduler API listening` and `swept
   leftovers of dead sandbox runs`, and `/healthz` answers `"status":"ok"` with every worker alive.
   Then run the [smoke test](#smoke-test).
10. **First administrator.** Registration only makes students, so create the account that appoints
    lecturers (FR-19). The password is read without echo and passed in the environment, never on a
    command line; `scheduler/README.md`, "First administrator", has the rules.

    ```sh
    ADMIN_NAME='Full Name'; ADMIN_EMAIL='admin@example.ac.id'
    IFS= read -rsp 'Administrator password: ' ADMIN_PASSWORD && echo && export ADMIN_PASSWORD
    sudo --preserve-env=ADMIN_PASSWORD -u grader \
        DATABASE_URL='postgres://grader@/autograder?host=/var/run/postgresql' \
        /usr/local/bin/autograder seed-admin --name "$ADMIN_NAME" --email "$ADMIN_EMAIL"
    printf '{"email":"%s","password":"%s"}' "$ADMIN_EMAIL" "$ADMIN_PASSWORD" |
        curl -s -o /dev/null -w 'login %{http_code}\n' -H 'Content-Type: application/json' \
        --data-binary @- http://127.0.0.1:8080/api/v1/auth/login
    unset ADMIN_PASSWORD
    ```

    Put the real name and address on the first line. A good run logs `administrator created` and
    prints `login 200`. The password is typed once, with no confirmation, and a second seed is
    refused, so the login is what catches a typo; `printf` is a shell builtin, so the password stays
    out of the process list, but it must not contain `"` or `\`, which would break the JSON. `IFS=`
    keeps spaces at either end of the password, which `read` would otherwise drop. Use a password of
    its own, not one of the host's: these are the credentials the handover (SRS Bagian 13.5) passes
    on.

### Backend environment

| Variable | Required | Meaning |
|---|---|---|
| `DATABASE_URL` | yes | `postgres://` URL, for example `postgres://grader@/autograder?host=/var/run/postgresql` for peer authentication over the Unix socket. The backend applies the migrations at start-up, so this user must be able to create tables in the database. |
| `JWT_SECRET` | yes | At least 32 random bytes; signs the access tokens. |
| `HTTP_ADDR` | yes | Listen address, for example `127.0.0.1:8080` behind Nginx. |
| `WORKERS` | no | Submissions graded at once, default 4 (SRS Bagian 5.1). `0` runs the API alone and grades nothing. |
| `CODE_DIR` | unless `WORKERS=0` | Absolute path of the stored submission files, for example `/var/lib/autograder/submissions`. |
| `HARNESS_COMMAND` | unless `WORKERS=0` | The Grading Harness program and its leading arguments, split on spaces, the program as an absolute path: `/opt/grader/venv/bin/python -m harness`. |
| `SANDBOX_EXEC` | no | Path of `sandbox-exec`, default `/usr/local/bin/sandbox-exec`, the path the AppArmor profile names. |

The Harness gets none of these: its environment is only `SANDBOX_EXEC`, `SANDBOX_CGROUP_ROOT` and
`HARNESS_PROGRESS_FD`, which the backend sets.

## Smoke test

After an install or an upgrade, push three submissions through the installed pipeline:

```sh
sudo -v && infra/smoke-test.sh
infra/smoke-test.sh clean
```

It registers and logs in a student through the API, then, because there is no upload endpoint yet,
adds a course, an assignment with two test cases and three submissions with SQL, and waits up to 90
seconds for the workers (the queue is filled from the database once a minute). It checks:

- A correct program scores 100/100, a wrong one 0/100 with both results `FAILED`, and one that calls
  `socket()` 0/100 with both results `SECURITY_VIOLATION` and two `SANDBOX_VIOLATION` audit rows for
  syscall 41.
- Memory comes from the cgroup (`CGROUP_PEAK`), `/healthz` answers 200 before and after, and the
  WebSocket endpoint is mounted.
- No `sbx-*` cgroup is left under the unit, and the log has no `ERROR` line.

It exits 0 only if every check passes. Its rows are recognisable (emails `smoke-*@example.test`, course
`SMOKE`); `clean` removes exactly those and the three stored files. A second run refuses until `clean`
has run. Run it on a database without real data, or run `clean` straight away.

To check that `/healthz` notices a dead database (NFR-09, SRS Bagian 11.6), stop PostgreSQL for a
moment: it must answer `503` with `"database":"unreachable"`, and `200` again once PostgreSQL is back,
without a restart of the backend.

```sh
sudo systemctl stop postgresql; sleep 2; curl -s -w ' %{http_code}\n' http://127.0.0.1:8080/healthz
sudo systemctl start postgresql; sleep 3; curl -s -w ' %{http_code}\n' http://127.0.0.1:8080/healthz
```

## Database backup

`autograder-backup.timer` dumps the database every day at 02:30 WIB and keeps the dumps of the last
14 days (SRS Bagian 13.4, NFR-13). Each one is a `pg_dump` archive in the custom format,
`/var/backups/autograder/autograder-<UTC time>.dump`, mode `0600`. The unit runs as `postgres`, not as
`grader`: the backend's account is the one a damaged or compromised backend would act as, so it must
not be able to change or delete the backups.

```sh
sudo install -m 0755 infra/db-backup.sh /usr/local/sbin/autograder-backup
sudo install -d -m 0700 -o postgres -g postgres /var/backups/autograder
sudo install -m 0644 infra/systemd/autograder-backup.service infra/systemd/autograder-backup.timer /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl start autograder-backup && sudo journalctl -u autograder-backup -n 4 -o cat
sudo systemctl enable --now autograder-backup.timer && systemctl list-timers autograder-backup.timer
```

The `start` takes a backup straight away and logs `backup written: /var/backups/autograder/...dump (N
bytes)`; `list-timers` shows the next run.

- A dump is written under a `.partial` name and renamed once `pg_dump` has succeeded, and older dumps
  are deleted only after that, so a failed run keeps every earlier backup. It shows in `systemctl
  --failed`, and the journal has `pg_dump`'s error.
- Each run deletes the dumps whose UTC date is 14 or more days before its own, so the daily timer
  keeps 14. Files in the directory that the script did not name are left alone.
- Before an upgrade that brings a migration (SRS Bagian 13.3), take one by hand with `sudo systemctl
  start autograder-backup`. It is deleted with the daily ones after 14 days; copy it elsewhere to keep
  it until the release is stable.
- A dump holds the `autograder` database only. The `grader` role is outside it (recreate it as in
  step 6), and so are the stored submission files under `CODE_DIR`.

**Checking a backup.** A backup that has never been restored does not count (SRS Bagian 13.4).
Restore the newest one into a scratch database, look at it, and drop it:

```sh
f=$(sudo -u postgres sh -c 'ls /var/backups/autograder/autograder-*.dump | tail -1') && echo "$f"
sudo -u postgres createdb -O grader autograder_check
sudo -u postgres pg_restore --exit-on-error -d autograder_check "$f"
sudo -u postgres psql -d autograder_check -c 'SELECT version, dirty FROM schema_migrations' -c 'SELECT count(*) FROM users'
sudo -u postgres dropdb autograder_check
```

`pg_restore` prints nothing on success, and the schema version is the one the backend logged at
start-up.

**Restoring.** Stop the backend, replace the database with the dump, and start it again:

```sh
sudo systemctl stop autograder
sudo -u postgres dropdb autograder
sudo -u postgres createdb -O grader autograder
sudo -u postgres pg_restore --exit-on-error -d autograder /var/backups/autograder/autograder-<UTC time>.dump
sudo systemctl start autograder
```

`dropdb` deletes everything written since that dump. If any of it might still be needed, take a backup
first, or restore into a database of another name and compare.

## Verifying the host

Delegation works differently for a system service than for the user scope that `sandbox/tests/run.sh`
uses by default, so run the adversarial suite once as a transient system service with the same
`Delegate=yes`. Use a non-root account that can read the checkout:

```sh
sudo systemd-run --wait --pipe --collect -p User="$USER" -p Delegate=yes \
    "$PWD/sandbox/tests/run.sh" /usr/local/bin/sandbox-exec
```

Expected: `Ran N tests ... OK`, where N is the size of the suite (64 at the time of writing), above
the summary systemd prints at the end (`Finished with result: success`, CPU time, memory peak).
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

- The install steps as one script, and Node.js on the host for the frontend build.
- An Nginx reverse proxy with TLS that serves the frontend build and uses `/healthz` as a health
  check. The unit has no watchdog until the backend reports readiness.
- Expiry of submission files (SRS Bagian 13.4, NFR-13).
- A copy of the daily backups off the server: SRS Bagian 13.4 stores them outside the application
  server, and they are on it for now. The restore test on staging (same section) waits for the
  staging host.

Secrets are never stored here. They live in GitHub Actions secrets, separately for each environment.
