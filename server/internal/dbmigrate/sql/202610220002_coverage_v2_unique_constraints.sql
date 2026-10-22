-- Databases bootstrapped from the 000000000001 baseline got the coverage v2
-- tables without the unique constraints from 202609020002 (its CREATE TABLE IF
-- NOT EXISTS was skipped). Batch creation and signal upserts use ON CONFLICT on
-- these columns, so analysis could never start there. Databases that ran
-- 202609020002 directly already have them; this is a no-op for them.
DO $$
DECLARE
  item record;
BEGIN
  FOR item IN
    SELECT * FROM (VALUES
      ('coverage_batches', 'uq_coverage_batches_logical', 'workspace_id, window_start, window_end, analyzer_version, policy_version'),
      ('coverage_batches', 'uq_coverage_batches_workspace_id', 'workspace_id, id'),
      ('coverage_analysis_attempts', 'uq_coverage_attempt_number', 'workspace_id, logical_work_key, attempt'),
      ('coverage_analysis_attempts', 'uq_coverage_attempt_workspace_id', 'workspace_id, id'),
      ('coverage_findings', 'uq_coverage_findings_workspace_id', 'workspace_id, id'),
      ('coverage_topics', 'uq_coverage_topics_key', 'workspace_id, canonical_key'),
      ('coverage_topics', 'uq_coverage_topics_workspace_id', 'workspace_id, id'),
      ('coverage_assignment_attempts', 'uq_coverage_assignment_attempt', 'workspace_id, finding_id, attempt'),
      ('coverage_assignment_attempts', 'uq_coverage_assignment_workspace_id', 'workspace_id, id'),
      ('coverage_unreviewed_signals', 'uq_coverage_signal_key', 'workspace_id, signal_key')
    ) AS constraints(table_name, constraint_name, columns)
  LOOP
    IF to_regclass(item.table_name) IS NOT NULL
      AND NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = item.constraint_name)
      AND to_regclass(item.constraint_name) IS NULL THEN
      EXECUTE format('ALTER TABLE %I ADD CONSTRAINT %I UNIQUE (%s)', item.table_name, item.constraint_name, item.columns);
    END IF;
  END LOOP;
END $$;
