import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import { agentRunArtifactLabel } from '@/components/pm/agentRunConstants';

const CUSTOMER_SURFACES = [
  'src/pages/automation/Agents.tsx',
  'src/pages/automation/CustomAgentCreatePanel.tsx',
  'src/components/pm/AgentRunTable.tsx',
  'src/components/pm/TaskDeliveryTimeline.tsx',
  'src/components/pm/CodingSession/CodingSessionHeader.tsx',
  'src/components/pm/CodingSession/CodingTranscriptPane.tsx',
  'src/components/pm/AgentRunArtifactView.tsx',
  'src/components/command-bar/CommandRunTimeline.tsx',
  'src/pages/PublicSharedView.tsx',
];

const FORBIDDEN_PRESENTATION_PATTERNS = [
  /AgentModelPill/,
  /CuratedModelSelect/,
  /data-coding-session-runtime-pill/,
  /AGENT_RUNTIME_LABELS\s*\[/,
  />\s*AI Provider\s*</,
  />\s*Execution (?:engine|Engine)\s*</,
  />\s*Runtime\s*</,
  /run\.session\.runtime_kind/,
];

describe('customer agent technical identity guard', () => {
  it.each(CUSTOMER_SURFACES)('%s does not present internal route or runtime controls', (file) => {
    const source = readFileSync(resolve(process.cwd(), file), 'utf8');
    for (const pattern of FORBIDDEN_PRESENTATION_PATTERNS) {
      expect(source, `${file} must not match ${pattern}`).not.toMatch(pattern);
    }
  });

  it('removes runtime prefixes from unknown artifact labels', () => {
    expect(agentRunArtifactLabel('codex_diagnostics')).toBe('diagnostics');
    expect(agentRunArtifactLabel('opencode_trace_bundle')).toBe('trace bundle');
  });
});
