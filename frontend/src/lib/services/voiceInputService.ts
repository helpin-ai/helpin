import { API_BASE, api, fetchWithSessionAuth } from '@/lib/api';

export async function voiceInputAvailable(workspaceId: string): Promise<boolean> {
  const result = await api.get<{ enabled: boolean }>(`/dock/transcriptions?workspace_id=${encodeURIComponent(workspaceId)}`);
  if (result.error) throw new Error(result.error);
  return result.data?.enabled === true;
}

export async function transcribeVoice(workspaceId: string, audio: Blob, signal: AbortSignal): Promise<string> {
  const response = await fetchWithSessionAuth(API_BASE, `/dock/transcriptions?workspace_id=${encodeURIComponent(workspaceId)}`, {
    method: 'POST', body: audio, signal, headers: { 'Content-Type': 'audio/wav' },
  });
  const result = await response.json().catch(() => null) as { text?: string; error?: string } | null;
  if (!response.ok) throw new Error(result?.error || 'Could not transcribe the recording. Please try again.');
  if (typeof result?.text !== 'string') throw new Error('Could not read the transcript. Please try again.');
  return result.text;
}
