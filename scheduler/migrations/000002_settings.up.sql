-- Settings an administrator can change (FR-14, US-20): the submission rate limits.
--
-- One row, enforced by the key: id is always 1. The row is created here with the values FR-14 gives, so
-- reading it never has to handle "no row". A limit of 0 switches that rule off. The upper bounds only
-- stop a typing mistake from becoming a limit nobody can reach (a day, ten thousand submissions).

CREATE TABLE settings (
	id                             smallint    PRIMARY KEY DEFAULT 1 CHECK (id = 1),
	-- At most this many submissions QUEUED or RUNNING per student and assignment.
	max_active_submissions         integer     NOT NULL DEFAULT 1
	                                           CHECK (max_active_submissions BETWEEN 0 AND 10000),
	-- Minimum seconds between two submissions by the same student to the same assignment.
	min_submission_interval_s      integer     NOT NULL DEFAULT 30
	                                           CHECK (min_submission_interval_s BETWEEN 0 AND 86400),
	-- At most this many submissions per student and assignment; cancelled ones do not count.
	max_submissions_per_assignment integer     NOT NULL DEFAULT 30
	                                           CHECK (max_submissions_per_assignment BETWEEN 0 AND 10000),
	created_at                     timestamptz NOT NULL DEFAULT now(),
	updated_at                     timestamptz NOT NULL DEFAULT now()
);

INSERT INTO settings DEFAULT VALUES;

CREATE TRIGGER settings_set_updated_at BEFORE UPDATE ON settings
	FOR EACH ROW WHEN (OLD.* IS DISTINCT FROM NEW.*) EXECUTE FUNCTION set_updated_at();
