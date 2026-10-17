-- Reverses 000003_submission_attempts.up.sql.
ALTER TABLE submissions DROP COLUMN IF EXISTS attempts;
