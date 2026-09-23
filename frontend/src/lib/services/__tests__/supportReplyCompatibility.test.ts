import { afterEach, describe, expect, it, vi } from 'vitest';
import { api } from '@/lib/api';
import { supportService } from '../supportService';
import type { CreateMessageRequest } from '@/lib/pmTypes';

vi.mock('@/lib/api', () => ({ api: { post: vi.fn() } }));
afterEach(() => vi.resetAllMocks());

const payload = { content: 'Hello', client_message_id: 'f97ecb2b-f331-4247-a0e8-122b2b72cb89' } as CreateMessageRequest;
const newPath = '/support/inbox/conversations/conversation/translation/sends?workspace_id=workspace';
const oldPath = '/support/inbox/conversations/conversation/messages?workspace_id=workspace';

describe('reply compatibility', () => {
  it('uses the legacy reply route when the new route is absent', async () => {
    vi.mocked(api.post).mockResolvedValueOnce({ data: null, error: 'Not Found', status: 404, isMissingRoute: true });
    vi.mocked(api.post).mockResolvedValueOnce({ data: { id: 'sent' } as never, error: null, status: 201 });
    const response = await supportService.sendConversationReply('workspace', 'conversation', payload);
    expect(response.data?.id).toBe('sent');
    expect(api.post).toHaveBeenNthCalledWith(1, newPath, payload);
    expect(api.post).toHaveBeenNthCalledWith(2, oldPath, payload);
  });

  it('does not retry a server error that may follow an accepted reply', async () => {
    vi.mocked(api.post).mockResolvedValueOnce({ data: null, error: 'Unavailable', status: 503 });
    await supportService.sendConversationReply('workspace', 'conversation', payload);
    expect(api.post).toHaveBeenCalledTimes(1);
  });

  it('does not bypass module gating or a resource 404', async () => {
    vi.mocked(api.post).mockResolvedValueOnce({ data: null, error: 'Module disabled', status: 404, isMissingRoute: false });
    await supportService.sendConversationReply('workspace', 'conversation', payload);
    expect(api.post).toHaveBeenCalledTimes(1);
  });
});
