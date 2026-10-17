-- Reverses 000001_initial_schema.up.sql. Dropping a table drops its triggers and indexes.
DROP TABLE IF EXISTS
	audit_logs,
	results,
	submissions,
	test_cases,
	resource_limits,
	assignments,
	enrollments,
	courses,
	refresh_tokens,
	users;

DROP FUNCTION IF EXISTS set_updated_at();
