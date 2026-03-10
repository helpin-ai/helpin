package repository

import (
	"fmt"

	"gorm.io/gorm"
)

// MigrateWorkspaceMemberSchema applies idempotent schema changes for the
// unified workspace_members identity model used by PM, settings, and rewards.
func MigrateWorkspaceMemberSchema(db *gorm.DB) error {
	const stmt = `
DO $$
BEGIN
    IF to_regclass('public.workspace_members') IS NOT NULL THEN
        IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'workspace_members' AND column_name = 'email') THEN
            ALTER TABLE workspace_members ADD COLUMN email text;
        END IF;
        IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'workspace_members' AND column_name = 'display_name') THEN
            ALTER TABLE workspace_members ADD COLUMN display_name text;
        END IF;
        IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'workspace_members' AND column_name = 'status') THEN
            ALTER TABLE workspace_members ADD COLUMN status text NOT NULL DEFAULT 'active';
        END IF;
        IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'workspace_members' AND column_name = 'invited_by') THEN
            ALTER TABLE workspace_members ADD COLUMN invited_by UUID REFERENCES users(id) ON DELETE SET NULL;
        END IF;
        IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'workspace_members' AND column_name = 'invited_at') THEN
            ALTER TABLE workspace_members ADD COLUMN invited_at TIMESTAMPTZ;
        END IF;
        IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'workspace_members' AND column_name = 'accepted_at') THEN
            ALTER TABLE workspace_members ADD COLUMN accepted_at TIMESTAMPTZ;
        END IF;

        ALTER TABLE workspace_members ALTER COLUMN user_id DROP NOT NULL;

        UPDATE workspace_members wm
        SET email = COALESCE(NULLIF(wm.email, ''), u.email),
            display_name = COALESCE(NULLIF(wm.display_name, ''), u.full_name),
            status = COALESCE(NULLIF(wm.status, ''), 'active'),
            invited_at = COALESCE(wm.invited_at, wm.created_at),
            accepted_at = COALESCE(wm.accepted_at, wm.created_at)
        FROM users u
        WHERE wm.user_id = u.id;

        UPDATE workspace_members
        SET email = COALESCE(NULLIF(email, ''), 'pending-' || id || '@placeholder.local'),
            display_name = COALESCE(NULLIF(display_name, ''), COALESCE(NULLIF(email, ''), 'Pending member')),
            status = COALESCE(NULLIF(status, ''), 'pending'),
            invited_at = COALESCE(invited_at, created_at)
        WHERE email IS NULL OR email = '' OR display_name IS NULL OR display_name = '';

        ALTER TABLE workspace_members ALTER COLUMN email SET NOT NULL;
        ALTER TABLE workspace_members ALTER COLUMN display_name SET NOT NULL;
        ALTER TABLE workspace_members ALTER COLUMN status SET DEFAULT 'active';

        CREATE UNIQUE INDEX IF NOT EXISTS idx_workspace_members_workspace_email_lower
            ON workspace_members (workspace_id, LOWER(email));
        CREATE INDEX IF NOT EXISTS idx_workspace_members_status
            ON workspace_members (workspace_id, status);
    END IF;

    IF to_regclass('public.workspace_invitations') IS NOT NULL THEN
        IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'workspace_invitations' AND column_name = 'workspace_member_id') THEN
            ALTER TABLE workspace_invitations ADD COLUMN workspace_member_id UUID REFERENCES workspace_members(id) ON DELETE SET NULL;
        END IF;

        UPDATE workspace_invitations wi
        SET workspace_member_id = wm.id
        FROM workspace_members wm
        WHERE wi.workspace_member_id IS NULL
          AND wi.workspace_id = wm.workspace_id
          AND LOWER(wi.email) = LOWER(wm.email);
    END IF;

    IF to_regclass('public.workspace_teams') IS NOT NULL THEN
        IF to_regclass('public.team_workspace_memberships') IS NULL THEN
            CREATE TABLE team_workspace_memberships (
                id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                team_id UUID NOT NULL REFERENCES workspace_teams(id) ON DELETE CASCADE,
                workspace_member_id UUID NOT NULL REFERENCES workspace_members(id) ON DELETE CASCADE,
                role TEXT NOT NULL DEFAULT 'member',
                created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
            );
        END IF;
        CREATE UNIQUE INDEX IF NOT EXISTS idx_team_workspace_member_unique
            ON team_workspace_memberships (team_id, workspace_member_id);
        CREATE INDEX IF NOT EXISTS idx_team_workspace_member_member
            ON team_workspace_memberships (workspace_member_id);
    END IF;

    IF to_regclass('public.reward_profiles') IS NULL THEN
        CREATE TABLE reward_profiles (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            workspace_member_id UUID NOT NULL REFERENCES workspace_members(id) ON DELETE CASCADE,
            manager_member_id UUID REFERENCES workspace_members(id) ON DELETE SET NULL,
            role TEXT NOT NULL DEFAULT 'employee',
            job_role TEXT NOT NULL DEFAULT '',
            hire_date TEXT,
            base_salary DOUBLE PRECISION NOT NULL DEFAULT 0,
            active_for_bonus BOOLEAN NOT NULL DEFAULT TRUE,
            active_for_evaluation BOOLEAN NOT NULL DEFAULT TRUE,
            is_account_owner BOOLEAN NOT NULL DEFAULT FALSE,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        );
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'reward_profiles' AND column_name = 'role') THEN
        ALTER TABLE reward_profiles ADD COLUMN role TEXT NOT NULL DEFAULT 'employee';
    END IF;
    CREATE UNIQUE INDEX IF NOT EXISTS idx_reward_profiles_workspace_member
        ON reward_profiles (workspace_member_id);
    CREATE INDEX IF NOT EXISTS idx_reward_profiles_manager_member
        ON reward_profiles (manager_member_id);

    IF to_regclass('public.pm_stories') IS NOT NULL THEN
        IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'pm_stories' AND column_name = 'owner_member_id') THEN
            ALTER TABLE pm_stories ADD COLUMN owner_member_id UUID REFERENCES workspace_members(id) ON DELETE SET NULL;
        END IF;
        IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'pm_stories' AND column_name = 'requester_member_id') THEN
            ALTER TABLE pm_stories ADD COLUMN requester_member_id UUID REFERENCES workspace_members(id) ON DELETE SET NULL;
        END IF;
        CREATE INDEX IF NOT EXISTS idx_pm_stories_owner_member_id ON pm_stories (owner_member_id);
        CREATE INDEX IF NOT EXISTS idx_pm_stories_requester_member_id ON pm_stories (requester_member_id);
    END IF;

    IF to_regclass('public.pm_epics') IS NOT NULL THEN
        IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'pm_epics' AND column_name = 'owner_member_id') THEN
            ALTER TABLE pm_epics ADD COLUMN owner_member_id UUID REFERENCES workspace_members(id) ON DELETE SET NULL;
        END IF;
        CREATE INDEX IF NOT EXISTS idx_pm_epics_owner_member_id ON pm_epics (owner_member_id);

        UPDATE pm_epics e
        SET owner_member_id = wm.id
        FROM workspace_members wm
        WHERE e.owner_member_id IS NULL
          AND e.owner_id IS NOT NULL
          AND wm.workspace_id = e.workspace_id
          AND wm.user_id = e.owner_id;
    END IF;

    IF to_regclass('public.pm_objectives') IS NOT NULL THEN
        IF to_regclass('public.pm_objective_owners') IS NOT NULL
           AND EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'pm_objective_owners' AND column_name = 'user_id')
           AND NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'pm_objective_owners' AND column_name = 'workspace_member_id') THEN
            ALTER TABLE pm_objective_owners RENAME TO pm_objective_owners_legacy;
        END IF;

        IF to_regclass('public.pm_objective_owners') IS NULL THEN
            CREATE TABLE pm_objective_owners (
                objective_id UUID NOT NULL REFERENCES pm_objectives(id) ON DELETE CASCADE,
                workspace_member_id UUID NOT NULL REFERENCES workspace_members(id) ON DELETE CASCADE,
                created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                PRIMARY KEY (objective_id, workspace_member_id)
            );
        END IF;
        CREATE INDEX IF NOT EXISTS idx_pm_objective_owners_workspace_member_id
            ON pm_objective_owners (workspace_member_id);

        IF to_regclass('public.pm_objective_owners_legacy') IS NOT NULL THEN
            INSERT INTO pm_objective_owners (objective_id, workspace_member_id, created_at)
            SELECT DISTINCT
                legacy.objective_id,
                wm.id,
                COALESCE(legacy.created_at, NOW())
            FROM pm_objective_owners_legacy legacy
            JOIN pm_objectives obj ON obj.id = legacy.objective_id
            JOIN workspace_members wm
              ON wm.workspace_id = obj.workspace_id
             AND wm.user_id = legacy.user_id
            ON CONFLICT (objective_id, workspace_member_id) DO NOTHING;

            DROP TABLE pm_objective_owners_legacy;
        END IF;
    END IF;
END $$;`

	if err := db.Exec(stmt).Error; err != nil {
		return fmt.Errorf("migrate workspace member schema: %w", err)
	}
	return nil
}

// DropLegacyWorkspaceIdentitySchema removes deprecated identity tables that
// were replaced by workspace_members, reward_profiles, and
// team_workspace_memberships.
func DropLegacyWorkspaceIdentitySchema(db *gorm.DB) error {
	const stmt = `
DO $$
DECLARE
    rec RECORD;
BEGIN
    IF to_regclass('public.workspace_people') IS NOT NULL THEN
        FOR rec IN
            SELECT conrelid::regclass AS table_name, conname
            FROM pg_constraint
            WHERE contype = 'f'
              AND confrelid = 'public.workspace_people'::regclass
        LOOP
            EXECUTE format('ALTER TABLE %s DROP CONSTRAINT IF EXISTS %I', rec.table_name, rec.conname);
        END LOOP;
    END IF;

    IF to_regclass('public.team_memberships') IS NOT NULL THEN
        FOR rec IN
            SELECT conrelid::regclass AS table_name, conname
            FROM pg_constraint
            WHERE contype = 'f'
              AND confrelid = 'public.team_memberships'::regclass
        LOOP
            EXECUTE format('ALTER TABLE %s DROP CONSTRAINT IF EXISTS %I', rec.table_name, rec.conname);
        END LOOP;
    END IF;

    IF to_regclass('public.team_user_memberships') IS NOT NULL THEN
        FOR rec IN
            SELECT conrelid::regclass AS table_name, conname
            FROM pg_constraint
            WHERE contype = 'f'
              AND confrelid = 'public.team_user_memberships'::regclass
        LOOP
            EXECUTE format('ALTER TABLE %s DROP CONSTRAINT IF EXISTS %I', rec.table_name, rec.conname);
        END LOOP;
    END IF;

    DROP TABLE IF EXISTS team_user_memberships CASCADE;
    DROP TABLE IF EXISTS team_memberships CASCADE;
    DROP TABLE IF EXISTS workspace_people CASCADE;
END $$;`

	if err := db.Exec(stmt).Error; err != nil {
		return fmt.Errorf("drop legacy workspace identity schema: %w", err)
	}
	return nil
}
