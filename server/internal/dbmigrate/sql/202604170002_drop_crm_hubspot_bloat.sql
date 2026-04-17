-- Migration B: drop CRM tables that duplicate capabilities the rest of the
-- platform already provides — property definitions, property groups, static
-- and smart lists, outbound sequences. The plan docs (see
-- docs/plans/2026-04-16-crm-pm-task-unification-plan.md) cover the rationale.
-- None of these surfaces shipped to the UI in a material way; custom fields
-- are still extensible through the existing `custom_properties` JSONB columns
-- on contacts / companies / deals.
--
-- Idempotent: CASCADE drops handle dependent FKs, IF EXISTS tolerates repeat
-- runs on databases where these tables were never created.

BEGIN;

DROP TABLE IF EXISTS crm_property_definitions CASCADE;
DROP TABLE IF EXISTS crm_property_groups CASCADE;

DROP TABLE IF EXISTS crm_list_members CASCADE;
DROP TABLE IF EXISTS crm_lists CASCADE;

DROP TABLE IF EXISTS crm_sequence_enrollments CASCADE;
DROP TABLE IF EXISTS crm_sequences CASCADE;

COMMIT;
