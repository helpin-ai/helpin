UPDATE support_coverage_gaps SET status = 'done'     WHERE status = 'fixed';
UPDATE support_coverage_gaps SET status = 'rejected' WHERE status = 'ignored';
UPDATE support_coverage_gaps SET status = 'open'     WHERE status = 'drafted';
