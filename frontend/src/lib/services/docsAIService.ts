import { API_BASE, fetchWithSessionAuth } from '@/lib/api'

export type DocsAIAction = 'improve' | 'fix_grammar' | 'shorten' | 'expand' | 'simplify' | 'professional' | 'friendly' | 'casual' | 'custom'

export async function rewriteDocsSelection(workspaceId: string, request: { content: string; operation: DocsAIAction; instructions?: string }, signal: AbortSignal): Promise<string> {
  const response = await fetchWithSessionAuth(API_BASE, `/docs/rewrite-selection?workspace_id=${encodeURIComponent(workspaceId)}`, {
    method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(request), signal,
  })
  const body = await response.json()
  if (!response.ok) throw new Error(typeof body.error === 'string' ? body.error : 'Could not rewrite the selected text. Try again.')
  if (typeof body.content !== 'string' || !body.content.trim()) throw new Error('AI returned no text. Try again.')
  return body.content
}
