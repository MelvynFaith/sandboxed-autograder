#!/bin/bash
# SPDX-License-Identifier: MIT
#
# An isolated database for the UAT (SRS Bagian 11.5; docs/uat/skenario-uat.md, "Gerbang sebelum
# mulai") on the campus server. Run it there as root (infra/README.md, "UAT database"):
#
#   sudo infra/uat-db.sh start [NAME]          new database, backend pointed at it, checked
#   sudo --preserve-env=ADMIN_PASSWORD infra/uat-db.sh seed NAME EMAIL
#                                              the first administrator of the UAT database
#   sudo infra/uat-db.sh status                what the backend uses, and the users in each database
#   sudo infra/uat-db.sh finish                save a dump, point the backend at production again
#   sudo infra/uat-db.sh drop NAME             delete a finished UAT database and its files
#
# The production database is never written to. `start` creates a database of its own (default
# uat_run_<date in WIB>) and a systemd drop-in that adds one more EnvironmentFile to the backend
# unit, with the UAT database and its own directory for submission files; a later EnvironmentFile
# wins over an earlier one. It then restarts the backend and proves which database the running
# backend is connected to, from the connections PostgreSQL itself lists. `finish` removes the
# drop-in and restarts the backend on production. While the UAT runs, the dashboard and the API
# serve the UAT database only, so no production account can log in, and the restart interrupts
# whatever is graded at that moment: do it when the platform is idle, which it is until real
# students use it.
#
# Overrides, for tests: UAT_UNIT, UAT_UNIT_DIR, UAT_ENV_DIR, UAT_CODE_BASE, UAT_DUMP_DIR,
# UAT_PG_SOCKET, UAT_API, UAT_BIN.
set -euo pipefail
cd /

unit=${UAT_UNIT:-autograder}
unit_dir=${UAT_UNIT_DIR:-/etc/systemd/system/$unit.service.d}
env_dir=${UAT_ENV_DIR:-/etc/autograder}
code_base=${UAT_CODE_BASE:-/var/lib/autograder/uat}
dump_dir=${UAT_DUMP_DIR:-/var/backups/autograder-uat}
socket=${UAT_PG_SOCKET:-/var/run/postgresql}
api=${UAT_API:-http://127.0.0.1:8080}
bin=${UAT_BIN:-/usr/local/bin/autograder}
prod_env=$env_dir/autograder.env
uat_env=$env_dir/uat.env
dropin=$unit_dir/uat.conf
self=infra/uat-db.sh

die() {
	echo "uat-db: $*" >&2
	exit 1
}

# setpriv leaves the environment alone, which `seed` needs for ADMIN_PASSWORD.
as_pg() { setpriv --reuid=postgres --regid=postgres --init-groups "$@"; }
as_grader() { setpriv --reuid=grader --regid=grader --init-groups "$@"; }
sql() { as_pg psql -X -qtA -v ON_ERROR_STOP=1 "$@"; }

# db_of_url FILE: the database named in the DATABASE_URL of an environment file.
db_of_url() { sed -n 's#^DATABASE_URL=[^@]*@[^/]*/\([^?]*\).*#\1#p' "$1" | head -1; }

# Names are put into SQL and into paths, so they are restricted, and "autograder" can never match.
valid_name() { [[ $1 =~ ^uat_[a-z0-9_]{1,40}$ ]]; }

uat_on() { [[ -e $uat_env || -e $dropin ]]; }

database_exists() { [[ -n $(sql -d postgres -c "SELECT 1 FROM pg_database WHERE datname = '$1'") ]]; }

# The databases the backend's role has connections to, one per line: the backend's own
# connections, as PostgreSQL lists them.
connected_to() {
	sql -d postgres -c "SELECT datname FROM pg_stat_activity WHERE usename = 'grader' AND datname IS NOT NULL GROUP BY datname ORDER BY datname"
}

wait_healthy() {
	local _
	for _ in $(seq 120); do
		if curl -fsS --max-time 2 "$api/healthz" 2> /dev/null | grep -q '"status":"ok"'; then
			return 0
		fi
		sleep 0.5
	done
	return 1
}

# undo_unit: production settings again, and the backend restarted on them.
undo_unit() {
	rm -f -- "$dropin" "$uat_env"
	rmdir --ignore-fail-on-non-empty -- "$unit_dir" 2> /dev/null || true
	systemctl daemon-reload
	systemctl restart "$unit"
}

# backend_must_use DB: the backend answers /healthz and is connected to DB and to no other database.
backend_must_use() {
	wait_healthy || return 1
	[[ $(connected_to) == "$1" ]]
}

cmd_start() {
	local db=${1:-uat_run_$(TZ=Asia/Jakarta date +%Y%m%d)} prod users code_dir
	valid_name "$db" || die "'$db' is not a name for a UAT database: it must be uat_ and 1 to 40 of a-z 0-9 _"
	! uat_on || die "UAT mode is already on (database $(db_of_url "$uat_env")); run '$self finish' first"
	[[ -r $prod_env ]] || die "cannot read $prod_env"
	prod=$(db_of_url "$prod_env")
	[[ -n $prod && $prod != "$db" ]] || die "cannot tell the production database from $prod_env"
	systemctl is-active --quiet "$unit" || die "$unit is not running; make it healthy first, so that a failure after the switch is not confused with this one"
	! database_exists "$db" || die "database $db exists already; pick another name, or '$self drop $db'"
	users=$(sql -d "$prod" -c 'SELECT count(*) FROM users')

	as_pg createdb -O grader "$db"
	[[ $(sql -d "$db" -c 'SELECT current_database()') == "$db" ]] || die "current_database() in $db does not say $db"
	code_dir=$code_base/$db
	install -d -m 0700 -o grader -g grader "$code_base" "$code_dir"

	install -d -m 0755 "$unit_dir"
	(
		umask 022
		printf '# The UAT run %s, written by infra/uat-db.sh; `uat-db.sh finish` removes it.\n# production users at start: %s\nDATABASE_URL=postgres://grader@/%s?host=%s\nCODE_DIR=%s\n' \
			"$db" "$users" "$db" "$socket" "$code_dir" > "$uat_env"
		printf '[Service]\nEnvironmentFile=%s\n' "$uat_env" > "$dropin"
	)
	systemctl daemon-reload
	systemctl restart "$unit"
	if ! backend_must_use "$db"; then
		echo "the backend did not come up on $db, or is connected elsewhere:" >&2
		connected_to >&2 || true
		journalctl -u "$unit" -n 15 -o cat --no-pager >&2 || true
		undo_unit
		die "went back to production; the empty database $db is still there"
	fi

	echo "UAT database:        $db (current_database() = $(sql -d "$db" -c 'SELECT current_database()'))"
	echo "schema:              version $(sql -d "$db" -c "SELECT version || (CASE WHEN dirty THEN ' dirty' ELSE ' clean' END) FROM schema_migrations" | tr -d '\n')"
	echo "backend connected to: $(connected_to | tr '\n' ' ')"
	echo "submission files in: $code_dir"
	echo "production database: $prod, $users users, not touched; no production account can log in until finish"
	echo "next: seed the first administrator of $db (infra/README.md, \"UAT database\")"
}

cmd_seed() {
	[[ $# -eq 2 ]] || die "usage: $self seed NAME EMAIL, with ADMIN_PASSWORD in the environment"
	uat_on || die "UAT mode is not on; run '$self start' first"
	[[ -n ${ADMIN_PASSWORD:-} ]] || die "ADMIN_PASSWORD is not set; read it with read -rsp and run sudo --preserve-env=ADMIN_PASSWORD"
	local db code
	db=$(db_of_url "$uat_env")
	(
		export DATABASE_URL="postgres://grader@/$db?host=$socket"
		as_grader "$bin" seed-admin --name "$1" --email "$2"
	)
	# printf is a builtin, so the password stays out of the process list.
	code=$(printf '{"email":"%s","password":"%s"}' "$2" "$ADMIN_PASSWORD" |
		curl -s -o /dev/null -w '%{http_code}' -H 'Content-Type: application/json' --data-binary @- "$api/api/v1/auth/login")
	echo "login $code"
	[[ $code == 200 ]] || die "the administrator was created but the login did not answer 200; check the password"
}

cmd_status() {
	local prod db
	prod=$(db_of_url "$prod_env")
	if uat_on; then
		db=$(db_of_url "$uat_env")
		echo "mode:        UAT, database $db (since $(stat -c %y "$uat_env" | cut -c1-19))"
	else
		echo "mode:        production"
	fi
	echo "backend connected to: $(connected_to | tr '\n' ' ')"
	echo "production:  $prod, $(sql -d "$prod" -c 'SELECT count(*) FROM users') users"
	local d
	for d in $(sql -d postgres -c "SELECT datname FROM pg_database WHERE datname LIKE 'uat\_%' ORDER BY datname"); do
		echo "UAT:         $d, $(sql -d "$d" -c 'SELECT count(*) FROM users') users, $(compgen -G "$dump_dir/$d-*.dump" | wc -l) saved dump(s)"
	done
}

cmd_finish() {
	uat_on || die "UAT mode is not on"
	local db prod users stamp dump
	db=$(db_of_url "$uat_env")
	valid_name "$db" || die "cannot read the UAT database from $uat_env"
	prod=$(db_of_url "$prod_env")
	users=$(sed -n 's/^# production users at start: //p' "$uat_env")
	stamp=$(date -u +%Y%m%dT%H%M%SZ)
	dump=$dump_dir/$db-$stamp.dump

	install -d -m 0700 -o postgres -g postgres "$dump_dir"
	as_pg pg_dump --format=custom --no-password -d "$db" --file="$dump.partial"
	as_pg mv -- "$dump.partial" "$dump"
	echo "saved: $dump ($(stat -c %s -- "$dump") bytes)"

	undo_unit
	backend_must_use "$prod" || die "the backend did not come back on $prod; see journalctl -u $unit"
	echo "backend connected to: $(connected_to | tr '\n' ' ')"
	echo "production database: $prod, $(sql -d "$prod" -c 'SELECT count(*) FROM users') users now, ${users:-?} at start"
	echo "$db and its submission files are kept for the report; '$self drop $db' deletes them"
}

cmd_drop() {
	[[ $# -eq 1 ]] || die "usage: $self drop NAME"
	local db=$1
	valid_name "$db" || die "'$db' is not a UAT database name"
	! uat_on || die "UAT mode is on; run '$self finish' first"
	database_exists "$db" || die "no database $db"
	compgen -G "$dump_dir/$db-*.dump" > /dev/null || die "no saved dump of $db in $dump_dir; '$self finish' saves one"
	[[ -z $(sql -d postgres -c "SELECT 1 FROM pg_stat_activity WHERE datname = '$db' LIMIT 1") ]] || die "$db has open connections"
	as_pg dropdb "$db"
	rm -rf -- "${code_base:?}/$db"
	echo "dropped $db and $code_base/$db; the dump stays in $dump_dir"
}

[[ $EUID -eq 0 ]] || die "run it as root: sudo $self ..."
case ${1:-} in
start) shift; cmd_start "$@" ;;
seed) shift; cmd_seed "$@" ;;
status) cmd_status ;;
finish) cmd_finish ;;
drop) shift; cmd_drop "$@" ;;
*) die "usage: $self start [NAME] | seed NAME EMAIL | status | finish | drop NAME" ;;
esac
