import type { AgentRun, AgentRuntimeKind } from './pmTypes';

const CODING_SESSION_RUNTIMES = new Set<AgentRuntimeKind>(['codex', 'opencode', 'native_sdk']);

export function supportsCodingSessionSurface(runtimeKind?: AgentRuntimeKind | null) {
  return !!runtimeKind && CODING_SESSION_RUNTIMES.has(runtimeKind);
}

export function canOpenCodingSession(run?: Pick<AgentRun, 'runtime_kind'> | null) {
  return supportsCodingSessionSurface(run?.runtime_kind);
}

export function buildCodingSessionPath(workspaceSlug?: string | null, sessionId?: string | null) {
  const slug = workspaceSlug?.trim();
  const id = sessionId?.trim();
  if (!slug || !id) return null;
  return `/w/${slug}/pm/coding-sessions/${id}`;
}
