CREATE TABLE IF NOT EXISTS agent_team_access (
    agent_id uuid NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    team_id uuid NOT NULL REFERENCES workspace_teams(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (agent_id, team_id)
);

CREATE INDEX IF NOT EXISTS idx_agent_team_access_team_id ON agent_team_access(team_id);

INSERT INTO agent_team_access (agent_id, team_id)
SELECT id, team_id
  FROM agents
 WHERE team_id IS NOT NULL
   AND NOT EXISTS (
       SELECT 1
         FROM agent_team_access ata
        WHERE ata.agent_id = agents.id
          AND ata.team_id = agents.team_id
   );
