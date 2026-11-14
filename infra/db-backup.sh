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
# Off-server copy (infra/README.md, "Off-server copy"): the dump directory is readable by postgres
# only, so when BACKUP_EXPORT_DIR names an existing directory the new dump is also copied there, as
# a file that the group of that directory can read, and only the newest BACKUP_EXPORT_KEEP (3)
# copies stay. A machine that pulls the copies over ssh then does not need to read the dump
# directory. Without that directory nothing is exported.
#
# Overrides: BACKUP_DIR (/var/backups/autograder), BACKUP_KEEP_DAYS (14), BACKUP_EXPORT_DIR (none),
# BACKUP_EXPORT_KEEP (3), PGDATABASE (autograder) and the other libpq variables (PGHOST, PGPORT,
# PGUSER), which pg_dump reads itself.
set -euo pipefail

dir=${BACKUP_DIR:-/var/backups/autograder}
keep_days=${BACKUP_KEEP_DAYS:-14}
export_dir=${BACKUP_EXPORT_DIR:-}
export_keep=${BACKUP_EXPORT_KEEP:-3}
export PGDATABASE=${PGDATABASE:-autograder}

if ! [[ $keep_days =~ ^[1-9][0-9]*$ ]]; then
	echo "BACKUP_KEEP_DAYS must be a positive whole number of days, not '$keep_days'" >&2
	exit 2
fi
if ! [[ $export_keep =~ ^[1-9][0-9]*$ ]]; then
	echo "BACKUP_EXPORT_KEEP must be a positive whole number of copies, not '$export_keep'" >&2
	exit 2
fi

umask 077
stamp=$(date -u +%Y%m%dT%H%M%SZ)
tmp=$(mktemp "$dir/autograder-$stamp.XXXXXX.partial")
export_tmp=
# bash runs the EXIT trap on SIGTERM and SIGINT too, so systemctl stop removes the partial files.
trap 'rm -f -- "$tmp" ${export_tmp:+"$export_tmp"}' EXIT

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

# The copy is written under a .partial name first, like the dump, so a machine that pulls at that
# moment never takes half a file. The directory is meant to be setgid (infra/README.md), so the copy
# gets its group, and mode 0640 lets that group read it. A failure here fails the unit, but only
# after the dump is safe and the old dumps are gone, and it shows in `systemctl --failed` and in
# the journal.
if [[ -z $export_dir ]]; then
	exit 0
fi
if [[ ! -d $export_dir ]]; then
	echo "export skipped: $export_dir does not exist"
	exit 0
fi
export_tmp=$(mktemp "$export_dir/autograder-$stamp.XXXXXX.partial")
cp -- "$out" "$export_tmp"
chmod 0640 -- "$export_tmp"
mv -- "$export_tmp" "$export_dir/${out##*/}"
export_tmp=
echo "exported: $export_dir/${out##*/}"

copies=()
for f in "$export_dir"/autograder-*.dump; do
	if [[ ${f##*/} =~ ^autograder-[0-9]{8}T[0-9]{6}Z\.dump$ ]]; then
		copies+=("$f")
	fi
done
for ((i = 0; i < ${#copies[@]} - export_keep; i++)); do
	rm -f -- "${copies[i]}"
	echo "removed: ${copies[i]}"
done
