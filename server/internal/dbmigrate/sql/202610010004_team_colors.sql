ALTER TABLE workspace_teams
    ADD COLUMN IF NOT EXISTS color text
    CHECK (color IS NULL OR color ~ '^#[0-9a-f]{6}$');

-- Give existing teams a saved color, cycling through the palette per workspace.
-- Only unset colors are assigned, so retries preserve any saved choice.
WITH ranked_teams AS (
    SELECT id, ROW_NUMBER() OVER (
        PARTITION BY workspace_id ORDER BY LOWER(name), id
    ) - 1 AS color_index
    FROM workspace_teams
    WHERE color IS NULL
)
UPDATE workspace_teams AS team
SET color = CASE ranked.color_index % 8
    WHEN 0 THEN '#4e8fea'
    WHEN 1 THEN '#2da88e'
    WHEN 2 THEN '#45a557'
    WHEN 3 THEN '#c7a53d'
    WHEN 4 THEN '#e58c3a'
    WHEN 5 THEN '#e2564a'
    WHEN 6 THEN '#e54e78'
    WHEN 7 THEN '#8b5cf6'
END
FROM ranked_teams AS ranked
WHERE team.id = ranked.id;
