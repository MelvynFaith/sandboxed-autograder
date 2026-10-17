-- How many times a worker has started grading a submission since it was last queued or finished.
--
-- A submission that crashes the scheduler every time it is graded would otherwise be requeued at each
-- restart for ever. Claim adds one; Complete and Fail set it back to 0, so a regrade starts clean. Only
-- a submission that keeps being lost mid-grading accumulates attempts, and Reconcile gives up on it.

ALTER TABLE submissions ADD COLUMN attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0);
