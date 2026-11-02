-- AI usage periods, ledger, reservations, and settlements are billing history
-- (settlements reference Stripe invoices). They keep workspace_id as the
-- deleted workspace's archived id instead of blocking or cascading its delete.
-- Links between the usage tables stay in place.
DO $$
DECLARE c record;
BEGIN
  FOR c IN
    SELECT con.conrelid::regclass AS tbl, con.conname
      FROM pg_constraint con
     WHERE con.contype = 'f'
       AND con.confrelid = 'workspaces'::regclass
       AND con.conrelid IN (
         'billing_ai_usage_periods'::regclass, 'billing_ai_usage_ledger'::regclass,
         'billing_ai_usage_reservations'::regclass, 'billing_ai_usage_settlements'::regclass)
  LOOP
    EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %I', c.tbl, c.conname);
  END LOOP;
END $$;
