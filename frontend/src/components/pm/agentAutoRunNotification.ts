interface AutoRunStateChangeInput {
  fromStateId?: string | null;
  toStateId?: string | null;
  automatedStateIds: ReadonlySet<string>;
}

export function shouldNotifyAgentAutoRunStateChange({
  fromStateId,
  toStateId,
  automatedStateIds,
}: AutoRunStateChangeInput): boolean {
  return Boolean(
    fromStateId &&
      toStateId &&
      fromStateId !== toStateId &&
      automatedStateIds.has(toStateId),
  );
}

export function getAgentAutoRunStateChangeMessage(stateName: string): string {
  return `Agent will run automatically because this task moved to ${stateName}.`;
}

export function getAgentAutoRunStateChangeToastId(toStateId: string): string {
  return `agent-auto-run-state-change-${toStateId}`;
}
