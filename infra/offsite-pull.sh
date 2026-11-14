#!/bin/bash
# SPDX-License-Identifier: MIT
#
# Pulls the database dumps from the campus server to this machine (SRS Bagian 13.4, NFR-13;
# infra/README.md, "Off-server copy"). Run it on your own computer, in Git Bash, WSL or Linux, not
# on the server:
#
#   infra/offsite-pull.sh USER@HOST
#
# It logs in over ssh once (it may ask for the password), streams the dumps that the server's backup
# unit exports as one tar archive, checks each one is a pg_dump archive, and adds the ones this
# machine does not have yet to the destination folder. A file already there is never overwritten; a
# file with the same name and a different content is an error. Nothing is deleted. The server gets
# no credentials for this machine, so a damaged server cannot reach the copies.
#
# A dump holds accounts, password hashes and the hidden test cases (FR-03). The destination must be
# a git-ignored folder or outside any repository, and the script refuses anything else.
#
# Overrides: OFFSITE_SERVER (USER@HOST, if not given as the argument), OFFSITE_DEST (private/backups
# of this repository) and OFFSITE_REMOTE_DIR (/var/backups/autograder-export). The address of the
# server is not kept in the repository; give it here each time.
set -euo pipefail

server=${1:-${OFFSITE_SERVER:-}}
remote=${OFFSITE_REMOTE_DIR:-/var/backups/autograder-export}
dest=${OFFSITE_DEST:-"$(cd "$(dirname "$0")/.." && pwd)/private/backups"}
stale_hours=36

# The first character must not be "-": the value would be read as an ssh option.
if ! [[ $server =~ ^[A-Za-z0-9_][A-Za-z0-9._-]*@[A-Za-z0-9][A-Za-z0-9._:-]*$ ]]; then
	echo "usage: ${0##*/} USER@HOST" >&2
	exit 2
fi

umask 077
mkdir -p -- "$dest"
if git -C "$dest" rev-parse --is-inside-work-tree > /dev/null 2>&1 &&
	! git -C "$dest" check-ignore -q -- .; then
	echo "refusing: $dest is inside a git repository and is not ignored by it" >&2
	exit 1
fi

tmp=$(mktemp -d "$dest/.pull.XXXXXX")
trap 'rm -rf -- "$tmp"' EXIT

printf -v command 'cd %q && tar -cf - -- autograder-*.dump' "$remote"
# With pipefail, a failed login, a missing directory or a cut connection fails the whole line, and
# nothing has left $tmp yet.
ssh -- "$server" "$command" | tar -xf - -C "$tmp"

added=0
had=0
for f in "$tmp"/autograder-*.dump; do
	name=${f##*/}
	if [[ ! -s $f || $(head -c 5 -- "$f") != PGDMP ]]; then
		echo "not a pg_dump archive: $name" >&2
		exit 1
	fi
	if [[ -e $dest/$name ]]; then
		if ! cmp -s -- "$f" "$dest/$name"; then
			echo "$name differs from the copy already in $dest; not overwritten" >&2
			exit 1
		fi
		had=$((had + 1))
		continue
	fi
	mv -- "$f" "$dest/$name"
	echo "new: $name ($(stat -c %s -- "$dest/$name") bytes)"
	added=$((added + 1))
done
echo "$added new, $had already here, in $dest"

# The names carry the UTC time of the dump. An old newest dump means the server's backup stopped.
newest=
for f in "$dest"/autograder-*.dump; do
	[[ ${f##*/} =~ ^autograder-[0-9]{8}T[0-9]{6}Z\.dump$ ]] && newest=${f##*/}
done
if [[ -z $newest ]]; then
	echo "no dump in $dest" >&2
	exit 1
fi
t=${newest#autograder-}
t=${t%.dump}
age=$((($(date -u +%s) - $(date -u -d "${t:0:4}-${t:4:2}-${t:6:2} ${t:9:2}:${t:11:2}:${t:13:2}" +%s)) / 3600))
echo "newest: $newest, $age h old"
if ((age > stale_hours)); then
	echo "WARNING: the newest dump is over $stale_hours h old; check the server's autograder-backup timer" >&2
	exit 1
fi
