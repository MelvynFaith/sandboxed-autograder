#!/bin/bash
# SPDX-License-Identifier: MIT
#
# Accounts and an assignment for the load test (SRS Bagian 11.4; infra/loadtest/README.md).
#
#   export LOAD_PASSWORD=...; infra/loadtest/seed.sh    ACCOUNTS students in one assignment
#   infra/loadtest/seed.sh report                       how every load-test submission was graded
#   infra/loadtest/seed.sh reset                        remove the submissions, keep the accounts
#   infra/loadtest/seed.sh clean                        remove everything this script made
#
# FR-14 allows a student one submission in the queue per assignment, so N simultaneous submissions
# need N accounts. The students register through the API, so their passwords are hashed as for real
# users. LOAD_PASSWORD is theirs, and it is required because these accounts can run code on the
# server. The lecturer, course, enrollments, assignment and test cases are made with SQL. Every row
# is recognisable (emails load-*@example.test, course LOAD), and `clean` removes exactly those.
#
# Overrides: ACCOUNTS (100), API (http://127.0.0.1:8080), DB (autograder), CODE_DIR
# (/var/lib/autograder/submissions), AS_POSTGRES ("sudo -u postgres") and AS_GRADER ("sudo -u
# grader"), the prefixes that run psql and delete the stored files.
set -euo pipefail

ACCOUNTS=${ACCOUNTS:-100}
API=${API:-http://127.0.0.1:8080}
DB=${DB:-autograder}
CODE_DIR=${CODE_DIR:-/var/lib/autograder/submissions}
AS_POSTGRES=${AS_POSTGRES-sudo -u postgres}
AS_GRADER=${AS_GRADER-sudo -u grader}

# The five test cases of the model in SRS Bagian 5.1 (N=5); the program in submit.js doubles its
# input, so it scores 100.
TEST_CASES="('tc_01', E'1\n', E'2\n'), ('tc_02', E'21\n', E'42\n'), ('tc_03', E'-7\n', E'-14\n'),
	('tc_04', E'0\n', E'0\n'), ('tc_05', E'123456789\n', E'246913578\n')"
LOAD_ASSIGNMENT="SELECT a.id FROM assignments a JOIN courses c ON c.id = a.course_id
	WHERE c.code = 'LOAD'"
LOAD_SUBMISSIONS="SELECT id FROM submissions WHERE assignment_id IN ($LOAD_ASSIGNMENT)"

# q runs SQL and prints the bare result.
q() { $AS_POSTGRES psql -d "$DB" -v ON_ERROR_STOP=1 -qtAX -c "$1"; }

# reset deletes the load-test submissions with their results and audit rows, then their stored
# files.
reset() {
	if [ "$(q "SELECT count(*) FROM submissions WHERE id IN ($LOAD_SUBMISSIONS)
		AND status IN ('QUEUED', 'RUNNING')")" != 0 ]; then
		echo "submissions are still queued or running;" \
			"wait until '$0 report' shows none" >&2
		exit 1
	fi
	local paths
	paths=$($AS_POSTGRES psql -d "$DB" -v ON_ERROR_STOP=1 -qtAX <<-SQL
	BEGIN;
	CREATE TEMP TABLE load_submissions ON COMMIT DROP AS $LOAD_SUBMISSIONS;
	DELETE FROM audit_logs WHERE submission_id IN (SELECT id FROM load_submissions);
	DELETE FROM results WHERE submission_id IN (SELECT id FROM load_submissions);
	DELETE FROM submissions WHERE id IN (SELECT id FROM load_submissions) RETURNING code_path;
	COMMIT;
	SQL
	)
	# A stored name is relative to CODE_DIR, and no part of it starts with a dot, so it cannot
	# climb out.
	local n=0 path name='[A-Za-z0-9_-][A-Za-z0-9._-]*'
	while IFS= read -r path; do
		[ -n "$path" ] || continue
		if [[ $path =~ ^$name(/$name)*$ ]]; then
			$AS_GRADER rm -f -- "$CODE_DIR/$path"
			n=$((n + 1))
		else
			echo "left alone, not a stored file name: $path" >&2
		fi
	done <<< "$paths"
	echo "removed $n load-test submissions"
}

clean() {
	reset
	$AS_POSTGRES psql -d "$DB" -v ON_ERROR_STOP=1 -qX <<-SQL
	BEGIN;
	DELETE FROM test_cases WHERE assignment_id IN ($LOAD_ASSIGNMENT);
	DELETE FROM resource_limits WHERE assignment_id IN ($LOAD_ASSIGNMENT);
	DELETE FROM assignments WHERE id IN ($LOAD_ASSIGNMENT);
	DELETE FROM enrollments WHERE course_id IN (SELECT id FROM courses WHERE code = 'LOAD');
	DELETE FROM courses WHERE code = 'LOAD';
	DELETE FROM audit_logs WHERE user_id IN
		(SELECT id FROM users WHERE email LIKE 'load-%@example.test');
	DELETE FROM users WHERE email LIKE 'load-%@example.test';
	COMMIT;
	SQL
	echo "load-test accounts and assignment removed"
}

# report prints how the load-test submissions were graded and how long the server took from
# submitted_at to graded_at. It exits 0 only if every one was graded with full marks.
report() {
	q "SELECT 'assignment ' || id FROM ($LOAD_ASSIGNMENT) a"
	q "SELECT coalesce(string_agg(status || ' ' || n, ', ' ORDER BY status), 'no submissions')
		FROM (SELECT status, count(*) AS n FROM submissions WHERE id IN ($LOAD_SUBMISSIONS)
		GROUP BY status) t"
	q "SELECT 'server time, submitted to graded: avg ' || round(avg(t)::numeric, 2)
		|| ' s, p95 ' || round(percentile_cont(0.95) WITHIN GROUP (ORDER BY t)::numeric, 2)
		|| ' s, max ' || round(max(t)::numeric, 2) || ' s'
		FROM (SELECT extract(epoch FROM graded_at - submitted_at) AS t FROM submissions
		WHERE id IN ($LOAD_SUBMISSIONS) AND graded_at IS NOT NULL) t HAVING count(*) > 0"
	local bad
	bad=$(q "SELECT count(*) FROM submissions WHERE id IN ($LOAD_SUBMISSIONS)
		AND (status <> 'COMPLETED' OR total_score IS DISTINCT FROM max_score
		OR max_score <> 100)")
	if [ "$bad" != 0 ]; then
		echo "$bad submission(s) not COMPLETED with 100/100"
		exit 1
	fi
	echo "every submission COMPLETED with 100/100"
}

case "${1:-}" in
report | reset | clean)
	"$1"
	exit 0
	;;
'') ;;
*)
	echo "usage: $0 [report|reset|clean]" >&2
	exit 2
	;;
esac

password=${LOAD_PASSWORD:-}
if ! [[ $ACCOUNTS =~ ^[1-9][0-9]{0,2}$ ]]; then
	echo "ACCOUNTS must be a whole number from 1 to 999, not '$ACCOUNTS'" >&2
	exit 2
fi
# The password goes into a JSON string below, so it may not contain a quote or a backslash.
if [ "${#password}" -lt 12 ] || [[ $password == *[\"\\]* ]]; then
	echo "set LOAD_PASSWORD to the students' password: at least 12 characters, no \" or \\" \
		"(for example: export LOAD_PASSWORD=\$(openssl rand -base64 18))" >&2
	exit 2
fi
if [ "$(q "SELECT count(*) FROM courses WHERE code = 'LOAD'")" != 0 ]; then
	echo "the load-test data already exists; '$0 clean' removes it" >&2
	exit 1
fi

echo "registering $ACCOUNTS students through $API..."
for i in $(seq -f %03g "$ACCOUNTS"); do
	# printf is a builtin, so the password reaches curl on stdin, not on a command line.
	status=$(printf '{"name":"Load Student %s","email":"%s","password":"%s"}' \
		"$i" "load-$i@example.test" "$password" | curl -s -o /dev/null -w '%{http_code}' \
		-X POST -H 'Content-Type: application/json' --data-binary @- \
		"$API/api/v1/auth/register")
	if [ "$status" != 201 ]; then
		echo "registering load-$i@example.test failed with HTTP $status;" \
			"'$0 clean' removes the rest" >&2
		exit 1
	fi
done

assignment=$($AS_POSTGRES psql -d "$DB" -v ON_ERROR_STOP=1 -qtAX <<-SQL
BEGIN;
WITH lecturer AS (
	INSERT INTO users (name, email, password_hash, role)
	VALUES ('Load Lecturer', 'load-lecturer@example.test', 'x', 'LECTURER') RETURNING id
)
INSERT INTO courses (code, name, lecturer_id, term)
	SELECT 'LOAD', 'Load test', id, 'load' FROM lecturer;
INSERT INTO enrollments (course_id, user_id)
	SELECT c.id, u.id FROM courses c CROSS JOIN users u
	WHERE c.code = 'LOAD' AND u.email LIKE 'load-%@example.test' AND u.role = 'STUDENT';
INSERT INTO assignments (course_id, title, deadline_at, late_until_at)
	SELECT id, 'Load test', now() + interval '7 days', now() + interval '8 days'
	FROM courses WHERE code = 'LOAD';
INSERT INTO resource_limits (assignment_id, cpu_time_ms, memory_mb, max_processes, timeout_s,
	max_output_bytes, tmpfs_size_mb)
	SELECT id, 2000, 64, 5, 5, 1048576, 32 FROM ($LOAD_ASSIGNMENT) a;
INSERT INTO test_cases (assignment_id, external_id, input, expected_output, weight, match_mode)
	SELECT a.id, t.external_id, t.input, t.expected_output, 20, 'EXACT'
	FROM ($LOAD_ASSIGNMENT) a
	CROSS JOIN (VALUES $TEST_CASES) AS t (external_id, input, expected_output);
$LOAD_ASSIGNMENT;
COMMIT;
SQL
)
echo "load test ready: $ACCOUNTS students enrolled in assignment $assignment"
echo "next: k6 run -e ACCOUNTS=30 -e ASSIGNMENT=$assignment infra/loadtest/submit.js"
