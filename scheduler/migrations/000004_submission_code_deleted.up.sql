-- When the stored file of a submission was deleted, NULL while it is kept.
--
-- NFR-13 and SRS 13.4 delete a submission's file one academic year after its assignment closes, and
-- keep its grade for the records. The row stays and says the code is gone, so that reading the code
-- (FR-20) or regrading (FR-17) can say why instead of failing on a missing file.

ALTER TABLE submissions ADD COLUMN code_deleted_at timestamptz;
