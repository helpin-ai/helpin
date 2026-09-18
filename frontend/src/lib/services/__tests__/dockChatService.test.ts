import { beforeEach, describe, expect, it, vi } from 'vitest';

import { api } from '@/lib/api';
import { dockChatService } from '@/lib/services/dockChatService';

vi.mock('@/lib/api', () => ({
  api: {
    post: vi.fn(),
  },
}));

describe('dockChatService.createChat', () => {
  beforeEach(() => {
    vi.mocked(api.post).mockReset();
  });

  it('persists an execution choice on the initial chat creation request', () => {
    void dockChatService.createChat('workspace-1', '', undefined, undefined, true);

    expect(api.post).toHaveBeenCalledWith('/dock/chats?workspace_id=workspace-1', {
      title: '',
      execution_enabled: true,
    });
  });

  it('keeps new chats non-executing by default', () => {
    void dockChatService.createChat('workspace-1');

    expect(api.post).toHaveBeenCalledWith('/dock/chats?workspace_id=workspace-1', {
      title: '',
    });
  });
});
