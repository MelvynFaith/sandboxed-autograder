#!/bin/bash
# SPDX-License-Identifier: MIT
#
# Daily database backup (SRS Bagian 13.4), run by systemd/autograder-backup.timer as the postgres OS
# user and installed as /usr/local/sbin/autograder-backup (infra/README.md, "Database backup").
#
# It dumps the database with pg_dump in its custom format to BACKUP_DIR/autograder-<UTC time>.dump
# and then deletes the dumps of every day before the last BACKUP_KEEP_DAYS days, counted in UTC
# dates, so a daily run keeps exactly that many. The dump is written under a .partial name and
# renamed only once pg_dump has succeeded, so a failed or interrupted run never looks like a backup
# and never causes an older one to be deleted.
#
# Overrides: BACKUP_DIR (/var/backups/autograder), BACKUP_KEEP_DAYS (14), PGDATABASE (autograder)
# and the other libpq variables (PGHOST, PGPORT, PGUSER), which pg_dump reads itself.
set -euo pipefail

dir=${BACKUP_DIR:-/var/backups/autograder}
keep_days=${BACKUP_KEEP_DAYS:-14}
export PGDATABASE=${PGDATABASE:-autograder}

if ! [[ $keep_days =~ ^[1-9][0-9]*$ ]]; then
	echo "BACKUP_KEEP_DAYS must be a positive whole number of days, not '$keep_days'" >&2
	exit 2
fi

umask 077
stamp=$(date -u +%Y%m%dT%H%M%SZ)
tmp=$(mktemp "$dir/autograder-$stamp.XXXXXX.partial")
# bash runs the EXIT trap on SIGTERM and SIGINT too, so systemctl stop removes the partial file.
trap 'rm -f -- "$tmp"' EXIT

# --no-password: fail instead of waiting for a prompt nobody can answer. pg_dump fsyncs the file
# before it exits.
pg_dump --format=custom --no-password --file="$tmp"
out="$dir/autograder-$stamp.dump"
mv -- "$tmp" "$out"
echo "backup written: $out ($(stat -c %s -- "$out") bytes)"

# Every name this script creates starts with the UTC date of its run, leftovers of interrupted
# runs included; any other file in the directory is left alone.
oldest=$(date -u -d "$((keep_days - 1)) days ago" +%Y%m%d)
for f in "$dir"/autograder-*; do
	name=${f##*/}
	[[ $name =~ ^autograder-([0-9]{8})T[0-9]{6}Z\. ]] || continue
	if ((10#${BASH_REMATCH[1]} < 10#$oldest)); then
		rm -f -- "$f"
		echo "removed: $f"
	fi
done
