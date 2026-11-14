# Infrastructure

Provisioning and deployment for the two environments. Staging and production are installed from
the same scripts and differ only in their `.env` file, database, and domain.

| Environment | Host | Deployed from |
|---|---|---|
| Staging | Free-tier cloud VM | merges to `develop` |
| Production | Campus server, reachable from outside the campus network | merges to `main` |

**Status:** the AppArmor profile, the systemd unit, the install steps, a smoke test, a daily
database backup, the expiry of old submission files and the Nginx site that serves the dashboard
exist, and the backend runs on the campus server from these steps. The Nginx site is plain HTTP
only; HTTPS waits for a DNS name. On 12 Nov 2026 it was installed on the campus server (Ubuntu's
Nginx 1.24) from step 11, where the site answered as the checks of that step expect and the smoke
test passed through it. The same day, from a phone on mobile data (outside the campus network, SRS
risk R10), the login page and `/healthz` answered over HTTP. Everything under
[Not started](#not-started) does not exist.

## Contents

| Path | What it is | SRS |
|---|---|---|
| `apparmor/sandbox-exec` | AppArmor profile that lets `sandbox-exec` create user namespaces | R9 |
| `systemd/autograder.service` | Backend unit; delegates a cgroup subtree to the service user | R11, Bagian 13.2 |
| `nginx/autograder.conf` | Nginx site: serves the dashboard build, proxies `/api/`, `/ws/` and `/healthz` to the backend | Bagian 13.2 step 6, NFR-09 |
| `smoke-test.sh` | Three uploads through the installed pipeline, checked against their scores | Bagian 13.2, NFR-09, FR-04, FR-14 |
| `db-backup.sh`, `systemd/autograder-backup.{service,timer}` | Daily `pg_dump` with 14-day retention, and an export of the newest dumps for the off-server copy | Bagian 13.4, NFR-13 |
| `offsite-pull.sh` | Run on your own computer: pulls the exported dumps from the server over ssh | Bagian 13.4, NFR-13 |
| `systemd/autograder-expire.{service,timer}` | Daily deletion of submission files a year after their assignment closed | Bagian 13.4, NFR-13 |
| `uat-db.sh` | Run on the server: an isolated database for the UAT, with the backend pointed at it and back | Bagian 11.5, R6 |
| `loadtest/` | Load tests: seeded student accounts and k6 scripts, [their own README](loadtest/README.md) | Bagian 11.4, NFR-01a, NFR-01b, FR-13, FR-14 |

## Host setup

Run these on the target host, from a checkout of this repository. They were last run in this order on
Ubuntu 24.04 (the campus server).

1. **Packages.** libseccomp 2.5 or newer; Ubuntu 24.04 packages PostgreSQL 16, the version CI tests
   against.

   ```sh
   sudo apt install build-essential libseccomp-dev git postgresql python3-venv
   ```
2. **Service user.** `sudo useradd --system --no-create-home --shell /usr/sbin/nologin grader`
3. **Go, for building only.** Any machine with Go 1.27.4 or newer can build the backend (step 9);
   `scheduler/go.mod` names that release because earlier ones have two security issues in the
   standard library that `govulncheck` reports for the backend (`net/http` and `crypto/x509`). To
   build on the host without installing Go system-wide:

   ```sh
   curl -fsSLo /tmp/go.tgz https://go.dev/dl/go1.27.4.linux-amd64.tar.gz
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
   sudo journalctl -u autograder -n 20 -o cat --no-pager
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
11. **Dashboard and Nginx.** The backend serves only the API, so Nginx serves the dashboard and
    passes `/api/`, `/ws/` and `/healthz` on to the backend; the browser then sees one origin and
    the dashboard's relative URLs need no CORS. Build the dashboard on any machine with Node.js 20 or
    newer, and copy the result to the host:

    ```sh
    (cd frontend && npm ci && npm run build)
    scp -r frontend/dist USER@HOST:/tmp/autograder-dist
    ```

    On the host, install Nginx, put the build in place and enable the site. Ubuntu's own `default`
    site also listens on port 80, so remove it:

    ```sh
    sudo apt install nginx
    sudo rm -rf /var/www/autograder && sudo cp -r /tmp/autograder-dist /var/www/autograder
    sudo install -m 0644 infra/nginx/autograder.conf /etc/nginx/sites-available/autograder
    sudo ln -s ../sites-available/autograder /etc/nginx/sites-enabled/autograder
    sudo rm -f /etc/nginx/sites-enabled/default
    sudo nginx -t && sudo systemctl reload nginx
    ```

    `nginx -t` must print `syntax is ok` and `test is successful`. If `sudo ufw status` says the
    firewall is active, allow the site with `sudo ufw allow 80/tcp` (and never enable `ufw` without
    allowing SSH first). Then check the site from the host, and run the smoke test through Nginx:

    ```sh
    curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1/                 # 200, the dashboard
    curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1/history          # 200, index.html
    curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1/assets/none.js   # 404, not index.html
    curl -s http://127.0.0.1/healthz                                           # "status":"ok"
    sudo -v && API=http://127.0.0.1 infra/smoke-test.sh; infra/smoke-test.sh clean
    ```

    Open the host's address in a browser from another network to prove the site is reachable from
    outside the campus (SRS risk R10, Bagian 13.2 step 11). The site is plain HTTP until there is a
    DNS name: passwords and tokens cross the network unencrypted, so use only test or throwaway
    passwords until [HTTPS](#https) is in place. To publish a new dashboard build, repeat the
    `scp` and the `rm`/`cp` lines; no reload is needed.

### Backend environment

| Variable | Required | Meaning |
|---|---|---|
| `DATABASE_URL` | yes | `postgres://` URL, for example `postgres://grader@/autograder?host=/var/run/postgresql` for peer authentication over the Unix socket. The backend applies the migrations at start-up, so this user must be able to create tables in the database. |
| `JWT_SECRET` | yes | At least 32 random bytes; signs the access tokens. |
| `HTTP_ADDR` | yes | Listen address, for example `127.0.0.1:8080` behind Nginx. |
| `WORKERS` | no | Submissions graded at once, default 4 (SRS Bagian 5.1). `0` runs the API alone and grades nothing. |
| `CODE_DIR` | yes | Absolute path where uploaded submission files are stored, for example `/var/lib/autograder/submissions`; required even when `WORKERS=0` because the API accepts uploads. |
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

It registers three students through the API and logs them in. A lecturer, a course, the
enrolments and an assignment with two test cases are added with SQL, because registration only makes
students and no administrator password is at hand to appoint a lecturer. Each student then uploads
one program with `POST /api/v1/submissions`, and the script waits up to 90 seconds for the workers.
`API=http://127.0.0.1` runs the same checks through Nginx. It checks:

- Each upload answers `202`, and a second upload by the same student straight afterwards answers
  `429` (FR-14).
- A correct program scores 100/100, a wrong one 0/100 with both results `FAILED`, and one that calls
  `socket()` 0/100 with both results `SECURITY_VIOLATION` and two `SANDBOX_VIOLATION` audit rows for
  syscall 41.
- Memory comes from the cgroup (`CGROUP_PEAK`), `/healthz` answers 200 before and after, and the
  WebSocket endpoint is mounted.
- No `sbx-*` cgroup is left under the unit, and the log has no `ERROR` line.

It exits 0 only if every check passes. Its rows are recognisable (emails `smoke-*@example.test`, course
`SMOKE`); `clean` removes exactly those and their stored files. A second run refuses until `clean`
has run. Run it on a database without real data, or run `clean` straight away.

To check that `/healthz` notices a dead database (NFR-09, SRS Bagian 11.6), stop PostgreSQL for a
moment: it must answer `503` with `"database":"unreachable"`, and `200` again once PostgreSQL is back,
without a restart of the backend.

```sh
sudo systemctl stop postgresql; sleep 2; curl -s -w ' %{http_code}\n' http://127.0.0.1:8080/healthz
sudo systemctl start postgresql; sleep 3; curl -s -w ' %{http_code}\n' http://127.0.0.1:8080/healthz
```

## Upgrading and rolling back

An upgrade replaces the Grading Harness and the backend binary; the sandbox and the unit files only
when they changed (compare with `git diff --stat <old> <new> -- sandbox infra/systemd infra/apparmor`).
It ran this way on the campus server on 12 Nov 2026, from `develop` at `eeaa8eb`. Run it from the
checkout (`~/sandboxed-autograder`) as the user who owns it.

1. Take a backup, then update the checkout:

   ```sh
   sudo systemctl start autograder-backup && sudo -u postgres ls /var/backups/autograder | tail -2
   git fetch -q origin && git switch -q develop && git pull -q --ff-only && git log --oneline -1
   ```

2. Reinstall the Harness (`--no-deps` leaves the installed PyYAML alone) and check that it starts
   under the service user with an empty environment:

   ```sh
   sudo /opt/grader/venv/bin/pip install -q --upgrade --force-reinstall --no-deps "$PWD/grader"
   (cd / && sudo -u grader env -i /opt/grader/venv/bin/python -m harness --help | head -1)
   ```

3. Build the backend, keep the running binary as `autograder.prev`, install the new one and restart:

   ```sh
   (cd scheduler && CGO_ENABLED=0 ~/go-sdk/bin/go build -o autograder ./cmd/autograder)
   sudo cp /usr/local/bin/autograder /usr/local/bin/autograder.prev
   sudo install -m 0755 scheduler/autograder /usr/local/bin/autograder && rm scheduler/autograder
   sudo systemctl restart autograder; sleep 3; systemctl is-active autograder
   sudo journalctl -u autograder -n 8 --no-pager -o cat
   curl -s -w '  -> %{http_code}\n' http://127.0.0.1:8080/healthz
   ```

   The log shows the schema up to date, the delegated cgroup ready, the worker pool started, the API
   listening and the sweep done; `/healthz` answers `200` with every worker alive.

4. Run the [smoke test](#smoke-test).

**Rolling back.** If the new backend does not start or fails the smoke test, put the previous binary
back:

```sh
sudo install -m 0755 /usr/local/bin/autograder.prev /usr/local/bin/autograder && sudo systemctl restart autograder
sleep 3; systemctl is-active autograder; curl -s http://127.0.0.1:8080/healthz
```

On 12 Nov 2026 this was rehearsed on the campus server: with the previous binary installed the
service came back `active` and `/healthz` answered with all four workers alive; the new binary was
then installed again (its checksum was the same as before) and the smoke test passed with 21 checks.

What this rehearsal covered and what it did not:

- The previous binary ran against the same schema (version 4). If an upgrade brings a migration, the
  previous binary may not run on the migrated database: restore the dump taken in step 1 as well (see
  [Database backup](#database-backup)), after stopping the backend. This case was not rehearsed.
- Only `/healthz` and the start-up log were checked with the previous binary; its pipeline was not
  run (the smoke test uploads through an endpoint the previous build did not have).
- The previous Harness is not kept. The new one is meant to work with the older backend, because the
  progress output it adds is optional, but that combination was not run end to end.

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
sudo systemctl start autograder-backup && sudo journalctl -u autograder-backup -n 4 -o cat --no-pager
sudo systemctl enable --now autograder-backup.timer && systemctl list-timers autograder-backup.timer --no-pager
```

The `start` takes a backup straight away and logs `backup written: /var/backups/autograder/...dump (N
bytes)`; `list-timers` shows the next run.

- A dump is written under a `.partial` name and renamed once `pg_dump` has succeeded, and older dumps
  are deleted only after that, so a failed run keeps every earlier backup. It shows in `systemctl
  --failed`, and the journal has `pg_dump`'s error.
- Each run deletes the dumps whose UTC date is 14 or more days before its own, so the daily timer
  keeps 14. Files in the directory that the script did not name are left alone.
- Before an upgrade that brings a migration (SRS Bagian 13.3), take one by hand with `sudo systemctl
  start autograder-backup`. It is deleted with the daily ones after 14 days; keep it until the
  release is stable by pulling it off the server ([Off-server copy](#off-server-copy)).
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
start-up. This was done on the campus server on 12 Nov 2026 with that day's dump: `pg_restore`
exited 0, the schema was version 4 and not dirty, and the scratch database held the same single
user as the live one. The live database had more audit rows (6 against 1), which were written after
the dump was taken.

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

### Off-server copy

The dumps above sit on the same disk as the database, so one failed disk loses both. The backup unit
can also export its newest dumps to a second directory, and `infra/offsite-pull.sh`, run on your own
computer, copies them from there over ssh. The server holds no credentials for your computer, so a
damaged or compromised server cannot reach the copies it has already handed over. On 12 Nov 2026 each
dump on the campus server was about 40 KB while the database reported 8.6 MB on disk, so a year of
daily dumps is about 15 MB at that size, and more once the UAT and load-test data are in.

On the server, once. The first two lines make a group for the people who may read the dumps and add
your login to it; the group takes effect at your next login, so log out and in again. The export
directory is setgid, so every copy gets that group, and the script gives it mode `0640`:

```sh
sudo groupadd --system autograder-backup
sudo usermod -aG autograder-backup "$USER"
sudo install -d -m 2750 -o postgres -g autograder-backup /var/backups/autograder-export
sudo install -m 0755 infra/db-backup.sh /usr/local/sbin/autograder-backup
sudo install -m 0644 infra/systemd/autograder-backup.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl start autograder-backup && sudo journalctl -u autograder-backup -n 4 -o cat --no-pager
sudo ls -l /var/backups/autograder-export
```

The journal shows `exported: ...` and the listing a `-rw-r-----` file of group `autograder-backup`. The
unit exports only while that directory exists, so removing it turns the export off, and the export
keeps the newest three dumps (`BACKUP_EXPORT_KEEP`), not the 14 of the dump directory. If the export
itself fails, the unit is marked failed (`systemctl --failed`), but only after the dump is written
and the old dumps are deleted.

On your computer, in Git Bash, WSL or Linux, from a clone of this repository:

```sh
infra/offsite-pull.sh USER@HOST
```

It logs in once, so it may ask for the password. It streams the exported dumps as one tar archive,
checks that each is a `pg_dump` archive, and adds those it does not have to `private/backups/`, which
git ignores. It never overwrites or deletes a local file. It exits with an error if a file with the
same name has other content, if a file is not a dump, if the server exported nothing, or if the newest
dump is over 36 hours old, which means the server's backup timer stopped. `OFFSITE_DEST` chooses
another folder; the script refuses a folder inside a repository that does not ignore it, because a
dump holds accounts, password hashes and the hidden test cases (FR-03). The address of the server is
not kept in the repository.

- Run it at least every two days: the server exports the newest three dumps, which are three days
  when only the daily timer runs (a backup taken by hand pushes an older one out sooner). Dumps the
  server has already dropped are not recoverable from the export, only from the 14-day directory.
- A login in group `autograder-backup` can read every dump. The account that pulls is the
  administrator's, who has `sudo` and could read them anyway, so the group adds no access.
- Only the database is copied. The stored submission files under `CODE_DIR` and the `grader` role are
  not (see above), and a regrade (FR-17) needs the files.
- The pull checks only that a file is a `pg_dump` archive. Restore one into a scratch database as in
  "Checking a backup" (on the server, or on a computer with PostgreSQL) once after the first pull; a
  copy that has never been restored does not count (SRS Bagian 13.4).

## Submission file expiry

`autograder-expire.timer` runs `autograder expire-code` every day at 03:30 WIB as `grader`. It
deletes the stored files of submissions whose assignment closed (`late_until_at`) more than a year
ago and keeps their grades (SRS Bagian 13.4, NFR-13); `scheduler/README.md`, "Submission file
expiry", has the rules. The command is part of the backend binary, so rebuild and install that first
(step 9).

```sh
sudo install -m 0644 infra/systemd/autograder-expire.service infra/systemd/autograder-expire.timer /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl start autograder-expire; sudo journalctl -u autograder-expire -n 4 -o cat --no-pager
sudo systemctl enable --now autograder-expire.timer && systemctl list-timers autograder-expire.timer --no-pager
```

The `start` logs `deleted the files of expired submissions` with the numbers deleted, already
missing and failed, and the cut-off date. A failure makes the unit fail, so it shows in `systemctl
--failed`. To count the files due without deleting any, the manual check SRS Bagian 11.6 lists for
NFR-13, run the command with the same environment and `--dry-run`:

```sh
sudo systemd-run --wait --pipe --collect -p User=grader -p EnvironmentFile=/etc/autograder/autograder.env \
    /usr/local/bin/autograder expire-code --dry-run
```

## UAT database

The UAT of SRS Bagian 11.5 (`docs/uat/skenario-uat.md`) must run on a new database of its own, never
on the production one: its gate is a database such as `uat_run_YYYYMMDD`, selected before the server
starts and confirmed with `SELECT current_database()`. `uat-db.sh` does this on the campus server
without a second backend or another open port. It creates the database
and a directory for the UAT's submission files, adds a systemd drop-in that gives the backend unit one
more `EnvironmentFile` with the new `DATABASE_URL` and `CODE_DIR` (a later file overrides an earlier
one; the JWT secret and the rest stay), restarts the backend, and then checks from PostgreSQL's own
list of connections that the backend is connected to the UAT database and to no other. If the backend
does not answer `/healthz` or is connected elsewhere, it removes the drop-in, restarts the backend on
production again and fails. The production database is never written to.

On the server, from a checkout, with the platform idle:

```sh
sudo infra/uat-db.sh start
ADMIN_NAME='Full Name'; ADMIN_EMAIL='lecturer@example.ac.id'
IFS= read -rsp 'UAT administrator password: ' ADMIN_PASSWORD && echo && export ADMIN_PASSWORD
sudo --preserve-env=ADMIN_PASSWORD infra/uat-db.sh seed "$ADMIN_NAME" "$ADMIN_EMAIL"
unset ADMIN_PASSWORD
```

`start [NAME]` takes the name `uat_run_` and the date in WIB unless you give one (`uat_` and up to 40
of `a-z`, `0-9`, `_`); it prints the database, the schema version, the databases the backend is
connected to and the production user count, which is what the UAT report records (the database
name and this check, no credentials). `seed` runs the official `seed-admin` on the UAT database and
logs in to prove the password; it is the account that appoints the lecturer for the run, as in step
10. Give it a password of its own, not the handover one. Then run the scenario in the browser.
`status` shows at any time which database the backend uses, the user count of production and of
each UAT database, and the dumps saved.

```sh
sudo infra/uat-db.sh finish          # dump the UAT database, point the backend at production again
sudo infra/uat-db.sh drop uat_run_YYYYMMDD    # later, when the report no longer needs it
```

`finish` writes `/var/backups/autograder-uat/<name>-<UTC time>.dump` (`0700 postgres`), removes the
drop-in, restarts the backend and checks that it is connected to production only, printing the
production user count now and at the start. The UAT database and its files stay for the report. `drop`
refuses while the UAT is on, while the database has open connections, and when no dump of it was
saved; it deletes the database and `/var/lib/autograder/uat/<name>`, and the dump stays.

- While the UAT is on, the dashboard and the API serve the UAT database only, so no production
  account can log in, the administrator of the handover included. The restart at each end interrupts
  a submission that is being graded; do both when nothing is.
- The daily backup and the file expiry look at production only. The UAT database has no backup but the
  dump that `finish` takes.
- The administrator's password of the UAT database is not kept anywhere by the script; it exists only
  in the shell that typed it, and `seed` keeps it out of the process list.
- Tested with stubs for `systemctl`, `psql` and the rest (97 checks, 22 of 22 mutants of the script
  caught), with its SQL against a real PostgreSQL, and with a user systemd manager for the order of
  the two `EnvironmentFile` entries. It has not run on the campus server.

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

## HTTPS

Not done: it needs a DNS name for the host, and there is none yet. When there is one (`NAME` below),
issue a certificate with the webroot challenge the site already answers, then switch the site over
by following the lines marked `TLS` in `infra/nginx/autograder.conf`:

```sh
sudo apt install certbot
sudo install -d /var/www/letsencrypt
sudo certbot certonly --webroot -w /var/www/letsencrypt -d NAME --deploy-hook 'systemctl reload nginx'
```

Edit `/etc/nginx/sites-available/autograder` (the `listen` and `server_name` lines, the certificate
paths, and the second server that redirects port 80), run `sudo nginx -t && sudo systemctl reload
nginx`, and allow port 443 in the firewall. `certbot renew --dry-run` checks the renewal. If the
campus network administrators issue a certificate instead, point `ssl_certificate` and
`ssl_certificate_key` at theirs.

## Not started

- The install steps as one script.
- A watchdog on the unit and a health check of `/healthz` by the unit or Nginx: the backend does not
  report readiness to systemd yet.
- A copy of the daily backups off the server: SRS Bagian 13.4 stores them outside the application
  server, and they are on it for now. The restore test on staging (same section) waits for the
  staging host; the scratch restore on the campus server is described under
  [Database backup](#database-backup).

Secrets are never stored here. They live in GitHub Actions secrets, separately for each environment.
