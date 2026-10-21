import { useCallback, useEffect, useMemo } from 'react';
import { API_BASE, fetchWithSessionAuth } from '@/lib/api';

interface ImageRequest {
  controller: AbortController;
  promise: Promise<string>;
  url?: string;
}

export function useSupportImageContent(workspaceId?: string, conversationId?: string) {
  // A gallery shares recovered bytes between thumbnail, preview, and download.
  const requests = useMemo(() => new Map<string, ImageRequest>(), [workspaceId, conversationId]);
  useEffect(() => () => {
    for (const request of requests.values()) {
      request.controller.abort();
      if (request.url) URL.revokeObjectURL(request.url);
    }
    requests.clear();
  }, [requests]);

  const load = useCallback((attachmentId: string): Promise<string> => {
    const cached = requests.get(attachmentId);
    if (cached) return cached.promise;
    const controller = new AbortController();
    const timeout = window.setTimeout(() => controller.abort(), 15_000);
    const request: ImageRequest = {
      controller,
      promise: Promise.resolve(''),
    };
    request.promise = (async () => {
      try {
        const response = await fetchWithSessionAuth(API_BASE,
          `/support/inbox/conversations/${encodeURIComponent(conversationId!)}/attachments/${encodeURIComponent(attachmentId)}/content?workspace_id=${encodeURIComponent(workspaceId!)}`,
          { signal: controller.signal });
        if (!response.ok) throw new Error('Image unavailable');
        const blob = await response.blob();
        if (controller.signal.aborted || !blob.type.startsWith('image/')) throw new Error('Image unavailable');
        request.url = URL.createObjectURL(blob);
        return request.url;
      } catch (error) {
        if (requests.get(attachmentId) === request) requests.delete(attachmentId);
        throw error;
      } finally {
        window.clearTimeout(timeout);
      }
    })();
    requests.set(attachmentId, request);
    return request.promise;
  }, [workspaceId, conversationId, requests]);

  return workspaceId && conversationId ? load : undefined;
}
