-- Initial schema, from the entity table in SRS Bagian 7.1.
--
-- Conventions
--   * Table names are the plural of the SRS entity. Column names are the SRS attribute names, which
--     are identical in Go, Python and YAML.
--   * Surrogate keys are bigint identity columns. Timestamps are timestamptz, stored in UTC.
--   * Enumerations are text with a CHECK constraint, so adding a value is a plain constraint change.
--   * Every table except audit_logs has created_at and updated_at; a trigger keeps updated_at current.
--     audit_logs is append-only and has no updated_at.
--   * Foreign keys: the SRS fixes only Result.testcase_id (RESTRICT). The others follow one rule: a
--     child that has no meaning without its parent is deleted with it (CASCADE), a reference to
--     something that must outlive the row is RESTRICT, and an optional reference is SET NULL.

CREATE FUNCTION set_updated_at() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
	NEW.updated_at := now();
	RETURN NEW;
END
$$;

CREATE TABLE users (
	id            bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	name          text        NOT NULL CHECK (name <> ''),
	-- Lower-cased by the application; the CHECK keeps a plain unique index case-insensitive.
	email         text        NOT NULL UNIQUE CHECK (email = lower(email)),
	password_hash text        NOT NULL CHECK (password_hash <> ''),
	role          text        NOT NULL CHECK (role IN ('STUDENT', 'LECTURER', 'ADMIN')),
	created_at    timestamptz NOT NULL DEFAULT now(),
	updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE refresh_tokens (
	id         bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	user_id    bigint      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
	token_hash text        NOT NULL UNIQUE,
	expires_at timestamptz NOT NULL,
	revoked_at timestamptz,
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX refresh_tokens_user_id_idx ON refresh_tokens (user_id);

CREATE TABLE courses (
	id          bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	code        text        NOT NULL UNIQUE CHECK (code <> ''),
	name        text        NOT NULL CHECK (name <> ''),
	-- The user must have role LECTURER; the application checks that, the schema cannot.
	lecturer_id bigint      NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
	term        text        NOT NULL CHECK (term <> ''),
	created_at  timestamptz NOT NULL DEFAULT now(),
	updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX courses_lecturer_id_idx ON courses (lecturer_id);

CREATE TABLE enrollments (
	id          bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	course_id   bigint      NOT NULL REFERENCES courses (id) ON DELETE CASCADE,
	user_id     bigint      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
	enrolled_at timestamptz NOT NULL DEFAULT now(),
	created_at  timestamptz NOT NULL DEFAULT now(),
	updated_at  timestamptz NOT NULL DEFAULT now(),
	UNIQUE (course_id, user_id)
);
CREATE INDEX enrollments_user_id_idx ON enrollments (user_id);

-- Ownership flows through the course: there is no lecturer_id here (SRS 7.1).
CREATE TABLE assignments (
	id            bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	course_id     bigint      NOT NULL REFERENCES courses (id) ON DELETE RESTRICT,
	title         text        NOT NULL CHECK (title <> ''),
	description   text        NOT NULL DEFAULT '',
	deadline_at   timestamptz NOT NULL,
	-- Submissions up to this instant are accepted and flagged is_late; later ones are rejected.
	-- Equal to deadline_at when the assignment has no late period.
	late_until_at timestamptz NOT NULL,
	created_at    timestamptz NOT NULL DEFAULT now(),
	updated_at    timestamptz NOT NULL DEFAULT now(),
	CHECK (late_until_at >= deadline_at)
);
CREATE INDEX assignments_course_id_idx ON assignments (course_id);

-- One row per assignment. The invariants repeat the ones the scheduler validates when a lecturer
-- saves the limits and sandbox-exec checks again at start-up (SRS 3.2.1.2): tmpfs pages are charged
-- to the cgroup, so the tmpfs must fill before memory.max is reached.
CREATE TABLE resource_limits (
	id               bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	assignment_id    bigint      NOT NULL UNIQUE REFERENCES assignments (id) ON DELETE CASCADE,
	cpu_time_ms      integer     NOT NULL CHECK (cpu_time_ms > 0),
	memory_mb        integer     NOT NULL CHECK (memory_mb > 0),
	max_processes    integer     NOT NULL CHECK (max_processes > 0),
	timeout_s        integer     NOT NULL CHECK (timeout_s > 0),
	-- Combined stdout+stderr limit, in bytes. tmpfs_size_mb and memory_mb are in MiB.
	max_output_bytes integer     NOT NULL CHECK (max_output_bytes > 0),
	tmpfs_size_mb    integer     NOT NULL CHECK (tmpfs_size_mb > 0),
	created_at       timestamptz NOT NULL DEFAULT now(),
	updated_at       timestamptz NOT NULL DEFAULT now(),
	CHECK (2::bigint * max_output_bytes < tmpfs_size_mb::bigint * 1048576),
	CHECK (tmpfs_size_mb + 16 <= memory_mb)
);

CREATE TABLE test_cases (
	id              bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	assignment_id   bigint      NOT NULL REFERENCES assignments (id) ON DELETE CASCADE,
	-- The identifier used in the YAML import/export file, for example tc_01.
	external_id     text        NOT NULL CHECK (external_id <> ''),
	input           text        NOT NULL,
	expected_output text        NOT NULL,
	weight          integer     NOT NULL CHECK (weight > 0),
	is_hidden       boolean     NOT NULL DEFAULT false,
	-- A test case that has results is retired with is_deleted, never removed (SRS 7.1).
	is_deleted      boolean     NOT NULL DEFAULT false,
	match_mode      text        NOT NULL CHECK (match_mode IN ('EXACT', 'WHITESPACE_INSENSITIVE')),
	created_at      timestamptz NOT NULL DEFAULT now(),
	updated_at      timestamptz NOT NULL DEFAULT now(),
	UNIQUE (assignment_id, external_id)
);

-- status describes the grading pipeline, not the grade (SRS 7.1): a submission whose tests all fail
-- is COMPLETED with total_score 0.
CREATE TABLE submissions (
	id                bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	user_id           bigint      NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
	assignment_id     bigint      NOT NULL REFERENCES assignments (id) ON DELETE RESTRICT,
	-- Server-generated storage name. The name the client sent is data only.
	code_path         text        NOT NULL CHECK (code_path <> ''),
	original_filename text        NOT NULL,
	size_bytes        integer     NOT NULL CHECK (size_bytes >= 0),
	status            text        NOT NULL DEFAULT 'QUEUED'
	                              CHECK (status IN ('QUEUED', 'RUNNING', 'COMPLETED', 'FAILED', 'CANCELLED')),
	is_late           boolean     NOT NULL DEFAULT false,
	-- Set when a test case changes after the grade was written (SRS 7.1). No automatic regrade.
	stale_grade       boolean     NOT NULL DEFAULT false,
	-- Denormalised sums, written in the same transaction as the results. NULL until graded.
	total_score       integer,
	max_score         integer,
	submitted_at      timestamptz NOT NULL DEFAULT now(),
	graded_at         timestamptz,
	created_at        timestamptz NOT NULL DEFAULT now(),
	updated_at        timestamptz NOT NULL DEFAULT now(),
	CHECK ((total_score IS NULL) = (max_score IS NULL)),
	CHECK (total_score BETWEEN 0 AND max_score)
);
-- History per student and assignment, and the per-student rate limits (FR-14).
CREATE INDEX submissions_user_assignment_idx ON submissions (user_id, assignment_id, submitted_at DESC);
CREATE INDEX submissions_assignment_idx ON submissions (assignment_id);
-- Start-up reconciliation of stale RUNNING rows and reloading of the queue.
CREATE INDEX submissions_active_idx ON submissions (submitted_at) WHERE status IN ('QUEUED', 'RUNNING');

CREATE TABLE results (
	id            bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	submission_id bigint      NOT NULL REFERENCES submissions (id) ON DELETE CASCADE,
	testcase_id   bigint      NOT NULL REFERENCES test_cases (id) ON DELETE RESTRICT,
	status        text        NOT NULL CHECK (status IN (
	                              'PASSED', 'FAILED', 'TIMEOUT', 'MEMORY_LIMIT_EXCEEDED',
	                              'RUNTIME_ERROR', 'SYNTAX_ERROR', 'SECURITY_VIOLATION')),
	-- PostgreSQL text cannot hold a NUL byte, and a program can print one: sanitise the output
	-- before inserting it.
	actual_output text        NOT NULL DEFAULT '',
	-- Shown to the student, except for hidden test cases (FR-03). NULL when PASSED.
	error_message text,
	is_truncated  boolean     NOT NULL DEFAULT false,
	exec_time_ms  integer     NOT NULL CHECK (exec_time_ms >= 0),
	memory_kb     integer     NOT NULL CHECK (memory_kb >= 0),
	-- Where memory_kb came from (SRS 3.2.1.3); a RUSAGE figure is less precise than a cgroup one.
	memory_source text        NOT NULL CHECK (memory_source IN ('CGROUP_PEAK', 'CGROUP_SAMPLED', 'RUSAGE')),
	score         integer     NOT NULL CHECK (score >= 0),
	created_at    timestamptz NOT NULL DEFAULT now(),
	updated_at    timestamptz NOT NULL DEFAULT now(),
	UNIQUE (submission_id, testcase_id)
);
CREATE INDEX results_testcase_id_idx ON results (testcase_id);

-- Append-only. user_id is required: the log must say who did it, including violations that belong
-- to no submission (FR-12).
CREATE TABLE audit_logs (
	id            bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	user_id       bigint      NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
	submission_id bigint      REFERENCES submissions (id) ON DELETE SET NULL,
	event_type    text        NOT NULL CHECK (event_type <> ''),
	syscall_nr    integer     CHECK (syscall_nr >= 0),
	detail        text        NOT NULL DEFAULT '',
	created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_logs_created_at_idx ON audit_logs (created_at DESC, id DESC);
CREATE INDEX audit_logs_user_id_idx ON audit_logs (user_id);
CREATE INDEX audit_logs_submission_id_idx ON audit_logs (submission_id) WHERE submission_id IS NOT NULL;

-- Keep updated_at current. The WHEN clause skips updates that change nothing.
DO $$
DECLARE
	t text;
BEGIN
	FOREACH t IN ARRAY ARRAY[
		'users', 'refresh_tokens', 'courses', 'enrollments', 'assignments',
		'resource_limits', 'test_cases', 'submissions', 'results'
	] LOOP
		EXECUTE format(
			'CREATE TRIGGER %I BEFORE UPDATE ON %I FOR EACH ROW '
			'WHEN (OLD.* IS DISTINCT FROM NEW.*) EXECUTE FUNCTION set_updated_at()',
			t || '_set_updated_at', t);
	END LOOP;
END
$$;
