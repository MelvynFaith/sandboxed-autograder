#!/bin/bash
# SPDX-License-Identifier: MIT
#
# Smoke test of an installed backend (infra/README.md, "Smoke test"): three submissions through the
# real pipeline, the API, the Harness and sandbox-exec, checked against what they must score.
#
#   sudo -v && infra/smoke-test.sh          run it; exits 0 only if every check passes
#   infra/smoke-test.sh clean               remove what a run left in the database and CODE_DIR
#
# Three students register and log in through the API and upload their programs with
# POST /api/v1/submissions (one student each: FR-14 allows one queued submission per student and
# assignment). Only the lecturer side is made with SQL, because registration always yields STUDENT
# and there is no administrator password to promote one: the lecturer, course, enrollments,
# assignment and test cases. Every row it makes is recognisable (emails smoke-*@example.test, course
# SMOKE), and `clean` removes exactly those and the stored files. Run it on a host whose database
# holds no real data, or run `clean` straight after.
#
# Overrides: API (http://127.0.0.1:8080), DB (autograder), CODE_DIR (/var/lib/autograder/submissions),
# UNIT (autograder; empty skips the cgroup and log checks), AS_POSTGRES ("sudo -u postgres") and
# AS_GRADER ("sudo -u grader"), the prefixes that run psql and write the stored files.
set -euo pipefail

API=${API:-http://127.0.0.1:8080}
DB=${DB:-autograder}
CODE_DIR=${CODE_DIR:-/var/lib/autograder/submissions}
UNIT=${UNIT-autograder}
AS_POSTGRES=${AS_POSTGRES-sudo -u postgres}
AS_GRADER=${AS_GRADER-sudo -u grader}

LECTURER=smoke-lecturer@example.test
PASSWORD=smoke-test-password
WAIT_S=90

# q runs one SQL statement and prints the bare result.
q() { $AS_POSTGRES psql -d "$DB" -v ON_ERROR_STOP=1 -qtAX -c "$1"; }

clean() {
	# The stored files have server-generated names, so list them before their rows go.
	files=$(q "SELECT s.code_path FROM submissions s JOIN assignments a ON a.id = s.assignment_id
		JOIN courses c ON c.id = a.course_id WHERE c.code = 'SMOKE'")
	$AS_POSTGRES psql -d "$DB" -v ON_ERROR_STOP=1 -qX <<-'SQL'
	BEGIN;
	CREATE TEMP TABLE smoke_submissions ON COMMIT DROP AS
		SELECT s.id FROM submissions s JOIN assignments a ON a.id = s.assignment_id
		JOIN courses c ON c.id = a.course_id WHERE c.code = 'SMOKE';
	DELETE FROM audit_logs WHERE submission_id IN (SELECT id FROM smoke_submissions);
	DELETE FROM results WHERE submission_id IN (SELECT id FROM smoke_submissions);
	DELETE FROM submissions WHERE id IN (SELECT id FROM smoke_submissions);
	DELETE FROM test_cases WHERE assignment_id IN
		(SELECT a.id FROM assignments a JOIN courses c ON c.id = a.course_id WHERE c.code = 'SMOKE');
	DELETE FROM resource_limits WHERE assignment_id IN
		(SELECT a.id FROM assignments a JOIN courses c ON c.id = a.course_id WHERE c.code = 'SMOKE');
	DELETE FROM assignments WHERE course_id IN (SELECT id FROM courses WHERE code = 'SMOKE');
	DELETE FROM enrollments WHERE course_id IN (SELECT id FROM courses WHERE code = 'SMOKE');
	DELETE FROM courses WHERE code = 'SMOKE';
	DELETE FROM audit_logs WHERE user_id IN (SELECT id FROM users WHERE email LIKE 'smoke-%@example.test');
	DELETE FROM users WHERE email LIKE 'smoke-%@example.test';
	COMMIT;
	SQL
	for f in $files; do
		$AS_GRADER rm -f "$CODE_DIR/$f"
	done
	echo "smoke data removed"
}

if [ "${1:-}" = clean ]; then
	clean
	exit 0
fi
if [ -n "${1:-}" ]; then
	echo "usage: $0 [clean]" >&2
	exit 2
fi

failures=0
check() { # check DESCRIPTION GOT WANT
	if [ "$2" = "$3" ]; then
		printf 'ok    %s\n' "$1"
	else
		printf 'FAIL  %s: got %s, want %s\n' "$1" "$2" "$3"
		failures=$((failures + 1))
	fi
}
http() { curl -s -o /dev/null -w '%{http_code}' "$@"; }

if [ -n "$(q "SELECT 1 FROM courses WHERE code = 'SMOKE'
	UNION ALL SELECT 1 FROM users WHERE email LIKE 'smoke-%@example.test' LIMIT 1")" ]; then
	echo "an earlier run left its data; run '$0 clean' first" >&2
	exit 1
fi
since=$(date '+%Y-%m-%d %H:%M:%S')

check "/healthz before the run" "$(http "$API/healthz")" 200

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
printf 'print(int(input()) * 2)\n' > "$work/ok.py"
printf 'print(int(input()) * 3)\n' > "$work/wrong.py"
printf 'import socket\nsocket.socket(socket.AF_INET, socket.SOCK_STREAM)\n' > "$work/net.py"

json=(-H 'Content-Type: application/json')
# json_field NAME reads a top-level number or string out of a one-line JSON object.
json_field() { sed -n "s/.*\"$1\":\"\{0,1\}\([^\",}]*\).*/\1/p"; }

declare -A student token
for f in ok wrong net; do
	email=smoke-$f@example.test
	check "register student $f" "$(http -X POST "${json[@]}" "$API/api/v1/auth/register" \
		-d "{\"name\":\"Smoke $f\",\"email\":\"$email\",\"password\":\"$PASSWORD\"}")" 201
	token[$f]=$(curl -s -X POST "${json[@]}" "$API/api/v1/auth/login" \
		-d "{\"email\":\"$email\",\"password\":\"$PASSWORD\"}" | json_field access_token)
	check "log in student $f" "$([ -n "${token[$f]}" ] && echo 200 || echo none)" 200
	student[$f]=$(q "SELECT id FROM users WHERE email = '$email'")
done

lecturer=$(q "INSERT INTO users (name, email, password_hash, role)
	VALUES ('Smoke Lecturer', '$LECTURER', 'x', 'LECTURER') RETURNING id")
course=$(q "INSERT INTO courses (code, name, lecturer_id, term)
	VALUES ('SMOKE', 'Smoke test', $lecturer, 'smoke') RETURNING id")
for f in ok wrong net; do
	q "INSERT INTO enrollments (course_id, user_id) VALUES ($course, ${student[$f]})" > /dev/null
done
assignment=$(q "INSERT INTO assignments (course_id, title, deadline_at, late_until_at)
	VALUES ($course, 'smoke', now() + interval '1 day', now() + interval '2 days') RETURNING id")
q "INSERT INTO resource_limits (assignment_id, cpu_time_ms, memory_mb, max_processes, timeout_s,
	max_output_bytes, tmpfs_size_mb) VALUES ($assignment, 2000, 64, 5, 5, 1048576, 32)" > /dev/null
q "INSERT INTO test_cases (assignment_id, external_id, input, expected_output, weight, match_mode) VALUES
	($assignment, 'tc_01', E'21\n', E'42\n', 50, 'EXACT'),
	($assignment, 'tc_02', E'5\n', E'10\n', 50, 'EXACT')" > /dev/null

# upload F prints the HTTP status of the upload; the reply body is left in $work/reply.
upload() {
	curl -s -o "$work/reply" -w '%{http_code}' -H "Authorization: Bearer ${token[$1]}" \
		-F "assignment_id=$assignment" -F "file=@$work/$1.py;filename=main.py" "$API/api/v1/submissions"
}
declare -A sub
for f in ok wrong net; do
	check "upload $f is accepted (202)" "$(upload "$f")" 202
	sub[$f]=$(json_field id < "$work/reply")
done
# FR-14: the same student cannot queue another submission for the assignment so soon.
check "a second upload is refused (429)" "$(upload ok)" 429

echo "waiting up to ${WAIT_S}s for the workers..."
for _ in $(seq "$WAIT_S"); do
	[ "$(q "SELECT count(*) FROM submissions WHERE assignment_id = $assignment
		AND status IN ('QUEUED', 'RUNNING')")" = 0 ] && break
	sleep 1
done

# grade ID prints status total/max and the result statuses in test-case order.
grade() {
	q "SELECT s.status || ' ' || coalesce(s.total_score::text, '-') || '/' || coalesce(s.max_score::text, '-')
		|| ' ' || coalesce((SELECT string_agg(r.status, ',' ORDER BY r.testcase_id) FROM results r
		WHERE r.submission_id = s.id), '-') FROM submissions s WHERE s.id = $1"
}
check "correct program" "$(grade "${sub[ok]}")" "COMPLETED 100/100 PASSED,PASSED"
check "wrong output" "$(grade "${sub[wrong]}")" "COMPLETED 0/100 FAILED,FAILED"
check "socket() denied" "$(grade "${sub[net]}")" "COMPLETED 0/100 SECURITY_VIOLATION,SECURITY_VIOLATION"
check "audit rows for the denied syscall" "$(q "SELECT string_agg(event_type || ':' || syscall_nr, ','
	ORDER BY id) FROM audit_logs WHERE submission_id = ${sub[net]}")" "SANDBOX_VIOLATION:41,SANDBOX_VIOLATION:41"
check "memory from the cgroup" "$(q "SELECT string_agg(DISTINCT memory_source, ',') FROM results
	WHERE submission_id IN (${sub[ok]}, ${sub[wrong]}, ${sub[net]})")" CGROUP_PEAK
check "/healthz after the run" "$(http "$API/healthz")" 200
check "WebSocket endpoint mounted (plain GET refused)" "$(http "$API/ws/v1/submissions/${sub[ok]}")" 400

if [ -n "$UNIT" ]; then
	cg=/sys/fs/cgroup$(systemctl show -P ControlGroup "$UNIT")
	check "no sandbox cgroups left behind" "$(find "$cg" -maxdepth 1 -name 'sbx-*' | wc -l)" 0
	# The system journal needs root (or the adm group) to read; without it the count would be 0.
	check "no ERROR in the log" "$(sudo journalctl -u "$UNIT" --since "$since" --no-pager -o cat |
		grep -c '"level":"ERROR"' || true)" 0
fi

if [ "$failures" -ne 0 ]; then
	echo "smoke test FAILED: $failures check(s); the data is left for inspection, '$0 clean' removes it"
	exit 1
fi
echo "smoke test passed; '$0 clean' removes its data"
