-- Removes what infra/qa/adversarial_api.py, cmdline_probe.py and crash_probe.py create: the users
-- adv-*, probe-* and inj-*@autograder.test, their Courses, assignments, test cases, enrolments,
-- submissions, results and audit rows. It leaves the audit rows that the administrator's own role
-- changes wrote (they belong to the administrator), and it does not remove the stored submission
-- files: delete those by code_path first if the directory matters (see below).
--
--   sudo -u postgres psql -d autograder -v ON_ERROR_STOP=1 -f infra/qa/clean.sql
--
-- To list the files before they go:
--   SELECT s.code_path FROM submissions s JOIN users u ON u.id = s.user_id
--     WHERE u.email ~ '^(adv|probe|inj)-.*@autograder\.test$';
BEGIN;

CREATE TEMP TABLE qa_users ON COMMIT DROP AS
	SELECT id FROM users WHERE email ~ '^(adv|probe|inj)-.*@autograder\.test$';
CREATE TEMP TABLE qa_assignments ON COMMIT DROP AS
	SELECT a.id FROM assignments a JOIN courses c ON c.id = a.course_id
	WHERE c.lecturer_id IN (SELECT id FROM qa_users);
CREATE TEMP TABLE qa_submissions ON COMMIT DROP AS
	SELECT id FROM submissions
	WHERE user_id IN (SELECT id FROM qa_users) OR assignment_id IN (SELECT id FROM qa_assignments);

DELETE FROM audit_logs WHERE submission_id IN (SELECT id FROM qa_submissions)
	OR user_id IN (SELECT id FROM qa_users);
DELETE FROM results WHERE submission_id IN (SELECT id FROM qa_submissions);
DELETE FROM submissions WHERE id IN (SELECT id FROM qa_submissions);
DELETE FROM assignments WHERE id IN (SELECT id FROM qa_assignments); -- test cases and limits cascade
DELETE FROM courses WHERE lecturer_id IN (SELECT id FROM qa_users);   -- enrolments cascade
DELETE FROM users WHERE id IN (SELECT id FROM qa_users);              -- refresh tokens cascade

COMMIT;
