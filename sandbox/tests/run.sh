#!/bin/bash
# SPDX-License-Identifier: MIT
#
# Run the test suite, or any command, inside a delegated cgroup.
#
#   tests/run.sh /abs/path/to/sandbox-exec [unittest args]
#   tests/run.sh /abs/path/to/sandbox-exec --exec CMD...
#
# sandbox-exec moves its child between cgroups, which the kernel only allows
# if the caller can write cgroup.procs of the common ancestor; the tests
# therefore run in a cgroup the user owns (a transient systemd scope with
# Delegate=yes, no root needed). cgroup v2 also refuses to enable controllers
# on a cgroup that holds processes, so this shell moves into a leaf first.
set -eu

bin=$1
shift
here=$(cd "$(dirname "$0")" && pwd)

self_cgroup() { echo "/sys/fs/cgroup$(cut -d: -f3 /proc/self/cgroup)"; }

if [ ! -w "$(self_cgroup)/cgroup.procs" ]; then
	# systemd-run expands $VAR in its arguments; "$$" passes a literal "$" through.
	args=("$bin" "$@")
	exec systemd-run --user --scope --quiet -p Delegate=yes "$0" "${args[@]//\$/\$\$}"
fi

cg=$(self_cgroup)
mkdir -p "$cg/supervisor"
echo $$ > "$cg/supervisor/cgroup.procs"
echo "+memory +pids +cpu" > "$cg/cgroup.subtree_control"

export SBX_BIN="$bin"
export SBX_CGROUP_ROOT="$cg"

if [ "${1:-}" = "--exec" ]; then
	shift
	exec "$@"
fi
exec python3 "$here/adversarial.py" "$@"
