/**
 * Whether the execution picker must be locked. Enabling mid-run cannot widen
 * the run that is already executing, and disabling mid-run (or while a message
 * is still on its way to the server) would cancel the run the user just
 * started, so the setting only changes while the chat is idle or paused.
 */
export function resolveExecutionPickerDisabled(state: {
  changingExecution: boolean;
  sending: boolean;
  pendingEcho: boolean;
  runStatus: string | null | undefined;
}): boolean {
  if (state.changingExecution || state.sending || state.pendingEcho) return true;
  return state.runStatus === 'queued' || state.runStatus === 'running' || state.runStatus === 'pending';
}
