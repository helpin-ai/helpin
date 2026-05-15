import { describe, expect, it } from 'vitest';
import { agentTemplateSourceLabel } from '../agentTemplateLabels';

describe('agentTemplateSourceLabel', () => {
  it('uses the flow template stamp when present', () => {
    expect(agentTemplateSourceLabel({ template_key: 'release_notes_writer' })).toBe('from Release Notes Writer template');
  });

  it('falls back to legacy source template keys', () => {
    expect(agentTemplateSourceLabel({ source_template_key: 'security_triage' })).toBe('from Security Triage template');
  });

  it('humanizes unknown template keys', () => {
    expect(agentTemplateSourceLabel({ template_key: 'custom_docs_sweep' })).toBe('from Custom Docs Sweep template');
  });

  it('returns null for regular custom agents', () => {
    expect(agentTemplateSourceLabel({})).toBeNull();
  });
});
