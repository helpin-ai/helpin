-- Run with psql -v ON_ERROR_STOP=1 -f this-file against a disposable database.
BEGIN;
CREATE TABLE crm_pipelines (id uuid PRIMARY KEY, workspace_id uuid, name text, is_default boolean, default_commercial_motion text);
CREATE TABLE crm_pipeline_stages (id uuid PRIMARY KEY, pipeline_id uuid REFERENCES crm_pipelines(id), name text, stage_type text, position integer, probability integer);
CREATE TABLE crm_deals (id integer PRIMARY KEY, stage_id uuid REFERENCES crm_pipeline_stages(id), probability integer, updated_at timestamptz);
CREATE TABLE crm_suggestions (id integer PRIMARY KEY, workspace_id uuid, suggestion_type text, status text, context jsonb, updated_at timestamptz);
CREATE TABLE automation_rules (id integer PRIMARY KEY, workspace_id uuid, trigger_config jsonb, action_config jsonb);
INSERT INTO crm_pipelines
SELECT ('00000000-0000-0000-0000-' || lpad(n::text,12,'0'))::uuid,
       ('10000000-0000-0000-0000-' || lpad(n::text,12,'0'))::uuid,
       'Sales Pipeline', true, 'new_business' FROM generate_series(1,3) n;
INSERT INTO crm_pipeline_stages
SELECT ('20000000-0000-0000-0000-' || lpad((p.n*10+s.pos)::text,12,'0'))::uuid,
       ('00000000-0000-0000-0000-' || lpad(p.n::text,12,'0'))::uuid,
       s.name,s.kind,s.pos,s.prob
FROM generate_series(1,3) p(n) CROSS JOIN (VALUES
 (0,'Appointment Scheduled','open',20),(1,'Qualified to Buy','open',40),
 (2,'Presentation Scheduled','open',60),(3,'Decision Maker Bought-In','open',80),
 (4,'Contract Sent','open',90),(5,'Closed Won','won',100),(6,'Closed Lost','lost',0)
) s(pos,name,kind,prob);
-- A renamed stage identifies a customized pipeline that must be preserved.
UPDATE crm_pipeline_stages SET name='Consultation' WHERE id='20000000-0000-0000-0000-000000000030';
INSERT INTO crm_deals SELECT n, ('20000000-0000-0000-0000-' || lpad(n::text,12,'0'))::uuid, 37, '2026-01-01' FROM generate_series(10,16) n;
INSERT INTO crm_suggestions VALUES
 (1,'10000000-0000-0000-0000-000000000001','deal_advance','pending','{"target_stage_id":"20000000-0000-0000-0000-000000000012"}',now()),
 (2,'10000000-0000-0000-0000-000000000001','deal_advance','accepted','{"target_stage_id":"20000000-0000-0000-0000-000000000012"}',now());
INSERT INTO automation_rules VALUES (1,'10000000-0000-0000-0000-000000000001','{"filters":[{"value":"20000000-0000-0000-0000-000000000012"}]}','{"stage_id":"20000000-0000-0000-0000-000000000013"}');
\ir ../sql/20260912000102_crm_simple_sales_pipeline.sql
\ir ../sql/20260912000102_crm_simple_sales_pipeline.sql
DO $$
BEGIN
 IF (SELECT count(*) FROM crm_pipeline_stages) <> 17 THEN RAISE EXCEPTION 'stage count or idempotency failed'; END IF;
 IF (SELECT array_agg(name ORDER BY position) FROM crm_pipeline_stages WHERE pipeline_id='00000000-0000-0000-0000-000000000001') <> ARRAY['Lead','In Discussion','Proposal Sent','Won','Lost'] THEN RAISE EXCEPTION 'wrong stages'; END IF;
 IF (SELECT array_agg(probability::bigint ORDER BY position) FROM crm_pipeline_stages WHERE pipeline_id='00000000-0000-0000-0000-000000000002') <> ARRAY[20,50,80,100,0]::bigint[] THEN RAISE EXCEPTION 'wrong probabilities'; END IF;
 IF (SELECT name FROM crm_pipeline_stages WHERE id='20000000-0000-0000-0000-000000000030') <> 'Consultation' THEN RAISE EXCEPTION 'custom pipeline changed'; END IF;
 IF (SELECT count(*) FROM crm_deals) <> 7 OR EXISTS (SELECT 1 FROM crm_deals WHERE probability<>37 OR updated_at<>'2026-01-01'::timestamptz) THEN RAISE EXCEPTION 'deal data changed'; END IF;
 IF EXISTS (SELECT 1 FROM crm_deals WHERE id IN (11,12,13) AND stage_id<>'20000000-0000-0000-0000-000000000011') THEN RAISE EXCEPTION 'discussion mapping failed'; END IF;
 IF (SELECT s.stage_type FROM crm_deals d JOIN crm_pipeline_stages s ON s.id=d.stage_id WHERE d.id=15) <> 'won' OR (SELECT s.stage_type FROM crm_deals d JOIN crm_pipeline_stages s ON s.id=d.stage_id WHERE d.id=16) <> 'lost' THEN RAISE EXCEPTION 'outcome changed'; END IF;
 IF (SELECT status FROM crm_suggestions WHERE id=1)<>'superseded' OR (SELECT status FROM crm_suggestions WHERE id=2)<>'accepted' THEN RAISE EXCEPTION 'suggestion handling failed'; END IF;
 IF (SELECT action_config->>'stage_id' FROM automation_rules WHERE id=1)<>'20000000-0000-0000-0000-000000000011' OR (SELECT trigger_config#>>'{filters,0,value}' FROM automation_rules WHERE id=1)<>'20000000-0000-0000-0000-000000000011' THEN RAISE EXCEPTION 'automation references broken'; END IF;
END $$;
ROLLBACK;
