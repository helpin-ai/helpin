-- Merge duplicate user accounts created by the (now-fixed) case-sensitive email
-- lookup, and close the bug class at the database layer.
--
-- Context: users.email had case-sensitive uniqueness + a case-sensitive lookup,
-- so the same person could fork into multiple rows differing only by email case
-- (observed: hassan.khattak@ vs Hassan.khattak@). This migration merges EVERY
-- such group, not just one pair, then enforces case-insensitive uniqueness.
--
-- Per group of users sharing lower(email):
--   * survivor = MIN(id). This matches GetByEmail's ORDER BY id .First(), so the
--     survivor is exactly the account each person now logs into.
--   * every other account's references are repointed onto the survivor, then the
--     account is deleted. No data is lost — references are moved, not dropped.
--
-- SAFETY PROPERTIES:
--   * Atomic: the migrator wraps this file in one transaction — it either fully
--     completes or rolls back with ZERO writes. It cannot half-merge.
--   * Idempotent: after it runs there are no duplicate groups and the unique
--     index exists, so re-runs (and other environments) are clean no-ops.
--   * Reference-complete: repoints by VALUE across all uuid columns and
--     conventionally-named text columns — catching declared FKs, undeclared
--     "soft" refs (e.g. organization_members.user_id, organizations.owner_id),
--     and text-typed refs (e.g. docs.created_by) alike. A user id is globally
--     unique, so a value match is a genuine reference; over-matching a column
--     that never holds a user id simply updates 0 rows.
--   * Collision-safe: for every UNIQUE index that includes a ref column, the
--     losing account's colliding rows are deleted first (survivor's row wins;
--     loser-only rows are preserved and repointed). Children cascade.

DO $$
DECLARE
    grp      record;
    v_dup    uuid;
    ref      record;
    ui       record;
    v_other  text;
    v_moved  bigint;
    v_total  bigint;
BEGIN
    -- All columns that can hold a user id: every uuid column (except users' own
    -- PK) plus conventionally-named text/varchar columns.
    CREATE TEMP TABLE _user_refs ON COMMIT DROP AS
        SELECT c.table_schema AS sch, c.table_name AS tbl, c.column_name AS col, c.data_type AS dtype
          FROM information_schema.columns c
          JOIN information_schema.tables t
            ON t.table_schema = c.table_schema AND t.table_name = c.table_name
           AND t.table_type = 'BASE TABLE'
         WHERE c.table_schema NOT IN ('pg_catalog', 'information_schema')
           AND NOT (c.table_name = 'users' AND c.column_name = 'id')
           AND (
                c.data_type = 'uuid'
             OR (c.data_type IN ('text', 'character varying')
                 AND (c.column_name LIKE '%\_by'
                      OR c.column_name IN (
                          'user_id','owner_id','created_by','created_by_id','updated_by',
                          'actor_id','actor_user_id','author_id','invited_by','resolved_by',
                          'changed_by','performed_by','locked_by','scored_by','requester_id',
                          'recipient_id','to_user_id','assigned_to','override_applied_by')))
           );

    FOR grp IN
        -- min(id::text)::uuid == lowest id by uuid order (Postgres has no
        -- min(uuid) aggregate); canonical uuid text sorts identically to the
        -- uuid byte order used by GetByEmail's ORDER BY id .First().
        SELECT lower(email) AS lem, min(id::text)::uuid AS survivor
          FROM users
         GROUP BY lower(email)
        HAVING count(*) > 1
    LOOP
        FOR v_dup IN
            SELECT id FROM users WHERE lower(email) = grp.lem AND id <> grp.survivor
        LOOP
            RAISE NOTICE 'merging % -> % (email %)', v_dup, grp.survivor, grp.lem;
            v_total := 0;

            -- 1. Collision-aware dedup: for each ref column, and each UNIQUE
            --    index on its table that includes it, drop the loser's rows that
            --    collide with a survivor row on the index's other columns.
            FOR ref IN SELECT * FROM _user_refs LOOP
                FOR ui IN
                    SELECT (SELECT array_agg(a.attname ORDER BY k.ord)
                              FROM unnest(i.indkey) WITH ORDINALITY AS k(attnum, ord)
                              JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = k.attnum
                           ) AS cols
                      FROM pg_index i
                      JOIN pg_class c     ON c.oid = i.indrelid
                      JOIN pg_namespace n ON n.oid = c.relnamespace
                     WHERE i.indisunique
                       AND i.indpred IS NULL       -- skip partial indexes
                       AND 0 <> ALL (i.indkey)     -- skip expression indexes
                       AND n.nspname = ref.sch
                       AND c.relname = ref.tbl
                LOOP
                    IF ui.cols IS NULL OR NOT (ref.col = ANY (ui.cols)) THEN
                        CONTINUE;
                    END IF;
                    SELECT string_agg(format('s.%I = d.%I', c, c), ' AND ')
                      INTO v_other
                      FROM unnest(ui.cols) AS c
                     WHERE c <> ref.col;

                    EXECUTE format(
                        'DELETE FROM %I.%I d WHERE d.%I::text = $2 AND EXISTS '
                        '(SELECT 1 FROM %I.%I s WHERE s.%I::text = $1%s)',
                        ref.sch, ref.tbl, ref.col, ref.sch, ref.tbl, ref.col,
                        CASE WHEN coalesce(v_other, '') = '' THEN '' ELSE ' AND ' || v_other END
                    ) USING grp.survivor::text, v_dup::text;
                    GET DIAGNOSTICS v_moved = ROW_COUNT;
                    IF v_moved > 0 THEN
                        RAISE NOTICE '  dedup %.% (unique on %): dropped % row(s)', ref.sch, ref.tbl, ref.col, v_moved;
                    END IF;
                END LOOP;
            END LOOP;

            -- 2. Repoint every ref column from loser to survivor (by value).
            FOR ref IN SELECT * FROM _user_refs LOOP
                EXECUTE format(
                    'UPDATE %I.%I SET %I = %s WHERE %I::text = $2',
                    ref.sch, ref.tbl, ref.col,
                    CASE WHEN ref.dtype = 'uuid' THEN '$1::uuid' ELSE '$1' END,
                    ref.col
                ) USING grp.survivor::text, v_dup::text;
                GET DIAGNOSTICS v_moved = ROW_COUNT;
                v_total := v_total + v_moved;
                IF v_moved > 0 THEN
                    RAISE NOTICE '  repointed %.% (%): % row(s)', ref.sch, ref.tbl, ref.col, v_moved;
                END IF;
            END LOOP;

            -- 3. Remove the merged-away account.
            DELETE FROM users WHERE id = v_dup;
            RAISE NOTICE '  done: % refs repointed, duplicate deleted', v_total;
        END LOOP;
    END LOOP;

    -- 4. Normalize all emails to lowercase (no lower(email) collisions remain).
    UPDATE users SET email = lower(email) WHERE email <> lower(email);

    -- 5. Enforce case-insensitive uniqueness at the DB layer going forward.
    CREATE UNIQUE INDEX IF NOT EXISTS users_email_lower_key ON users (lower(email));
    RAISE NOTICE 'merge complete; lower(email) unique index ensured';
END $$;
