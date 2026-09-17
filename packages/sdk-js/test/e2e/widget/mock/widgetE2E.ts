import type { Page } from '@playwright/test'

export const WIDGET_KEY = 'test-widget-key'
export const WIDGET_HOST = 'widget.test'
export const CONVERSATION_ID = 'conv-1'
export const LINK_PREVIEW_URL = 'https://example.com/docs'

export type WidgetE2EController = {
  clearRequests: () => void
  clearSentMessages: () => void
  disconnect: () => void
  emit: (payload: unknown) => void
  getRequests: () => Array<{ method: string; url: string; body?: Record<string, unknown> }>
  getSentMessages: () => Array<{ type: string; data?: Record<string, unknown> }>
  getSocketCount: () => number
  isReady: boolean
  setSocketBehavior: (mode: 'open' | 'fail') => void
}

declare global {
  interface Window {
    __widgetE2E?: WidgetE2EController
    helpin?: (...args: unknown[]) => unknown
  }
}

export type InstallWidgetMockOptions = {
  deferSession?: boolean
  invalidStoredSession?: boolean
  persistedSession?: boolean
  unreadCount?: number
  widgetPosition?: 'bottom-left' | 'bottom-right'
}

export async function installWidgetMocks(page: Page, options: InstallWidgetMockOptions = {}) {
  // Storage uploads use XMLHttpRequest for progress; fetch mocks do not intercept it.
  await page.route(`https://${WIDGET_HOST}/uploads/**`, route => route.fulfill({
    status: 200,
    headers: { 'Access-Control-Allow-Origin': '*' },
    body: '',
  }))
  await page.addInitScript(({ widgetHost, widgetKey, unreadCount, conversationId, linkPreviewUrl, persistedSession, invalidStoredSession, widgetPosition, deferSession }) => {
    const patchedUserAgent = (navigator.userAgent || '').replace(/HeadlessChrome\/[\d.]+\s*/i, 'Chrome/122.0.0.0 ')
    Object.defineProperty(Navigator.prototype, 'userAgent', {
      configurable: true,
      get() {
        return patchedUserAgent
      },
    })
    Object.defineProperty(Navigator.prototype, 'webdriver', {
      configurable: true,
      get() {
        return false
      },
    })

    const previewPayload = {
      url: linkPreviewUrl,
      title: 'Example Docs',
      description: 'Preview generated for a shared bare URL.',
      site_name: 'Example',
      image_url: 'https://example.com/preview.png',
      host: 'example.com',
    }

    const state = {
      attachmentCounter: 1,
      attachments: new Map<string, {
        id: string
        file_key: string
        file_name: string
        file_type: string
        file_size: number
        url: string
      }>(),
      conversationId,
      customerEmail: '',
      customerName: 'Visitor Example',
      isAnonymous: true,
      sessionToken: 'session-1',
      shouldRejectStoredSession: Boolean(invalidStoredSession),
      conversations: [
        {
          id: conversationId,
          subject: 'Support thread',
          status: 'open',
          last_message: 'Initial message',
          updated_at: '2026-03-27T20:00:00Z',
          unread_count: unreadCount,
          opened_by_user_id: 'agent-1',
          opened_by_display_name: 'Alice Agent',
          opened_by_avatar_url: 'https://example.com/alice.png',
          opened_by_status: 'online',
        },
      ],
      messages: [
        {
          id: 'msg-1',
          conversation_id: conversationId,
          sender_type: 'customer',
          sender_display_name: 'Visitor Example',
          content: 'Initial message',
          is_internal: false,
          via_channel: 'widget',
          created_at: '2026-03-27T19:56:00Z',
          updated_at: '2026-03-27T19:56:00Z',
        },
      ],
      requests: [] as Array<{ method: string; url: string; body?: Record<string, unknown> }>,
      socketBehavior: 'open' as 'open' | 'fail',
    }

    const sessionExpiresAt = new Date(Date.now() + 30 * 24 * 60 * 60 * 1000).toISOString()
    const buildSessionPayload = () => ({
      session_token: state.sessionToken,
      expires_at: sessionExpiresAt,
      is_anonymous: state.isAnonymous,
      customer_email: state.customerEmail || null,
      conversations: state.conversations,
      messages: state.messages,
      active_teammate: {
        user_id: 'agent-1',
        name: 'Alice Agent',
        avatar_url: 'https://example.com/alice.png',
        status: 'online',
      },
    })

    const buildConversationMessagesPayload = () => ({
      messages: state.messages,
      active_teammate: {
        user_id: 'agent-1',
        name: 'Alice Agent',
        avatar_url: 'https://example.com/alice.png',
        status: 'online',
      },
    })

    const originalFetch = window.fetch.bind(window)

    window.fetch = async (input, init) => {
      const rawUrl = typeof input === 'string' ? input : input.url
      const url = new URL(rawUrl, window.location.href)
      const method = (init?.method || 'GET').toUpperCase()
      const body =
        typeof init?.body === 'string' && init.body.length > 0
          ? JSON.parse(init.body)
          : undefined

      if (url.host === widgetHost && url.pathname === '/widget/telemetry') return new Response(null, { status: 204 })

      if (url.host === widgetHost && url.pathname === '/widget/config' && method === 'GET') {
        return new Response(
          JSON.stringify({
            workspaceId: 'ws-1',
            workspaceName: 'Support',
            availableTeammates: [
              {
                userId: 'agent-1',
                name: 'Alice Agent',
                avatarUrl: 'https://example.com/alice.png',
                status: 'online',
              },
            ],
            branding: {
              primaryColor: '#2563eb',
              logoUrl: '',
              welcomeMessage: 'Hi there. How can we help?',
              widgetPosition,
              showBranding: true,
              launcherIcon: 'chat_bubble',
              colorScheme: 'light',
              buttonColor: '#2563eb',
              buttonIconColor: '#ffffff',
            },
            features: {
              aiEnabled: false,
              aiFirst: false,
              showTalkToHuman: true,
              escalationMessage: 'A teammate will join shortly.',
              fileUploads: true,
              preChatForm: true,
              requirePhone: false,
              csatRating: false,
              forceIdentify: false,
            },
            availability: {
              isOnline: true,
              statusText: 'Online now',
              replyTimeText: 'Usually replies in a few minutes',
            },
          }),
          {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          },
        )
      }

      if (url.host === widgetHost && url.pathname === `/widget/conversations/${conversationId}/transcript` && method === 'POST') {
        state.requests.push({ method, url: url.toString(), body })
        return new Response(
          JSON.stringify({
            success: true,
            message: `Transcript sent to ${body?.email || 'visitor@example.com'}`,
          }),
          {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          },
        )
      }

      if (url.host === widgetHost && url.pathname === '/widget/support/attachments' && method === 'POST') {
        state.requests.push({ method, url: url.toString(), body })
        const attachmentId = `att-${state.attachmentCounter++}`
        const fileName = typeof body?.file_name === 'string' ? body.file_name : 'attachment.bin'
        const fileType = typeof body?.content_type === 'string' ? body.content_type : 'application/octet-stream'
        const fileSize = typeof body?.file_size === 'number' ? body.file_size : 0
        const publicUrl = `https://${widgetHost}/files/${attachmentId}/${encodeURIComponent(fileName)}`
        state.attachments.set(attachmentId, {
          id: attachmentId,
          file_key: `${attachmentId}/${fileName}`,
          file_name: fileName,
          file_type: fileType,
          file_size: fileSize,
          url: publicUrl,
        })
        return new Response(
          JSON.stringify({
            attachment: {
              id: attachmentId,
            },
            upload_url: `https://${widgetHost}/uploads/${attachmentId}`,
            public_url: publicUrl,
          }),
          {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          },
        )
      }

      if (url.host === widgetHost && url.pathname.startsWith('/widget/support/attachments/') && method === 'PATCH') {
        state.requests.push({ method, url: url.toString(), body })
        return new Response(JSON.stringify({ success: true }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }

      if (url.host === widgetHost && url.pathname.startsWith('/uploads/') && method === 'PUT') {
        state.requests.push({ method, url: url.toString() })
        return new Response('', { status: 200 })
      }

      if (url.host === widgetHost && (url.pathname === '/widget/messages' || url.pathname === '/widget/typing' || url.pathname === '/widget/session/revoke')) {
        state.requests.push({ method, url: url.toString(), body })
        return new Response(JSON.stringify({ success: true }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }

      return originalFetch(input, init)
    }

    class MockWebSocket {
      static instances: MockWebSocket[] = []
      static sentMessages: Array<{ type: string; data?: Record<string, unknown> }> = []
      static readonly CONNECTING = 0
      static readonly OPEN = 1
      static readonly CLOSING = 2
      static readonly CLOSED = 3

      readonly url: string
      readyState = MockWebSocket.CONNECTING
      onopen: ((event: Event) => void) | null = null
      onmessage: ((event: MessageEvent<string>) => void) | null = null
      onclose: ((event: CloseEvent) => void) | null = null
      onerror: ((event: Event) => void) | null = null

      constructor(url: string) {
        this.url = url
        MockWebSocket.instances.push(this)
        queueMicrotask(() => {
          if (state.socketBehavior === 'fail') {
            this.readyState = MockWebSocket.CLOSED
            this.onerror?.(new Event('error'))
            this.onclose?.(new CloseEvent('close'))
            return
          }
          this.readyState = MockWebSocket.OPEN
          this.onopen?.(new Event('open'))
        })
      }

      send(data: string) {
        const frame = JSON.parse(data) as { type: string; data?: Record<string, unknown> }
        MockWebSocket.sentMessages.push(frame)

        switch (frame.type) {
          case 'session:create':
            if (deferSession) break
            queueMicrotask(() => {
              this.serverEmit({
                type: 'session:joined',
                data: buildSessionPayload(),
              })
              if (window.__widgetE2E) {
                window.__widgetE2E.isReady = true
              }
            })
            break
          case 'session:restore':
            if (state.shouldRejectStoredSession) {
              state.shouldRejectStoredSession = false
              queueMicrotask(() => {
                this.serverEmit({
                  type: 'session:error',
                  data: {
                    error: 'invalid session',
                  },
                })
              })
              break
            }
            queueMicrotask(() => {
              this.serverEmit({
                type: 'session:joined',
                data: buildSessionPayload(),
              })
              if (window.__widgetE2E) {
                window.__widgetE2E.isReady = true
              }
            })
            break
          case 'session:upgrade':
            state.isAnonymous = false
            state.customerEmail = typeof frame.data?.email === 'string' ? frame.data.email : state.customerEmail
            queueMicrotask(() => {
              this.serverEmit({
                type: 'session:upgraded',
                data: {},
              })
            })
            break
          case 'conversation:new':
            queueMicrotask(() => {
              this.serverEmit({
                type: 'conversation:created',
                data: { conversation_id: state.conversationId },
              })
            })
            break
          case 'message:send': {
            const content = typeof frame.data?.content === 'string' ? frame.data.content : ''
            const previewMatch = content.match(/https?:\/\/\S+/)
            const attachmentIds = Array.isArray(frame.data?.attachment_ids)
              ? frame.data.attachment_ids.filter((value): value is string => typeof value === 'string')
              : []
            const message = {
              id: `msg-${Date.now()}`,
              conversation_id: state.conversationId,
              sender_type: 'customer',
              sender_display_name: state.customerName,
              content,
              attachments: attachmentIds
                .map((attachmentId) => state.attachments.get(attachmentId))
                .filter((attachment): attachment is NonNullable<typeof attachment> => Boolean(attachment)),
              metadata: previewMatch
                ? {
                    link_previews: [
                      {
                        ...previewPayload,
                        url: previewMatch[0],
                      },
                    ],
                  }
                : undefined,
              is_internal: false,
              via_channel: 'widget',
              created_at: '2026-03-27T20:01:00Z',
            }
            state.messages.push(message)
            state.conversations = state.conversations.map((conversation) =>
              conversation.id === state.conversationId
                ? {
                    ...conversation,
                    last_message: content,
                    updated_at: message.created_at,
                  }
                : conversation,
            )
            queueMicrotask(() => {
              this.serverEmit({
                type: 'message:received',
                data: message,
              })
            })
            break
          }
          case 'conversations:list':
            queueMicrotask(() => {
              this.serverEmit({
                type: 'conversations:listed',
                data: { conversations: state.conversations },
              })
            })
            break
          case 'conversation:select':
            state.conversations = state.conversations.map((conversation) =>
              conversation.id === frame.data?.conversation_id
                ? { ...conversation, unread_count: 0 }
                : conversation,
            )
            queueMicrotask(() => {
              this.serverEmit({
                type: 'conversation:messages',
                data: buildConversationMessagesPayload(),
              })
            })
            break
          case 'conversation:read':
            state.conversations = state.conversations.map((conversation) =>
              conversation.id === frame.data?.conversation_id
                ? { ...conversation, unread_count: 0 }
                : conversation,
            )
            break
          default:
            break
        }
      }

      close() {
        this.serverClose()
      }

      serverClose() {
        this.readyState = MockWebSocket.CLOSED
        this.onclose?.(new CloseEvent('close'))
      }

      serverEmit(payload: unknown) {
        this.onmessage?.(new MessageEvent('message', { data: JSON.stringify(payload) }))
      }
    }

    Object.defineProperty(window, 'WebSocket', {
      value: MockWebSocket,
      writable: true,
    })

    window.__widgetE2E = {
      isReady: false,
      clearRequests: () => {
        state.requests = []
      },
      clearSentMessages: () => {
        MockWebSocket.sentMessages = []
      },
      disconnect: () => {
        MockWebSocket.instances.at(-1)?.serverClose()
      },
      emit: (payload) => {
        MockWebSocket.instances.at(-1)?.serverEmit(payload)
      },
      getRequests: () => [...state.requests],
      getSentMessages: () => [...MockWebSocket.sentMessages],
      getSocketCount: () => MockWebSocket.instances.length,
      setSocketBehavior: (mode) => {
        state.socketBehavior = mode
      },
    }

    try {
      localStorage.removeItem(`helpin_wc_${widgetKey}`)
      localStorage.removeItem(`helpin_ws_${widgetKey}`)
      if (persistedSession || invalidStoredSession) {
        localStorage.setItem(`helpin_ws_${widgetKey}`, JSON.stringify({
          session_token: 'session-1',
          expires_at: sessionExpiresAt,
        }))
      }
    } catch {
      // ignore localStorage issues in tests
    }
  }, {
    deferSession: options.deferSession,
    widgetHost: WIDGET_HOST,
    widgetKey: WIDGET_KEY,
    persistedSession: options.persistedSession ?? false,
    invalidStoredSession: options.invalidStoredSession ?? false,
    unreadCount: options.unreadCount ?? 0,
    widgetPosition: options.widgetPosition ?? 'bottom-right',
    conversationId: CONVERSATION_ID,
    linkPreviewUrl: LINK_PREVIEW_URL,
  })
}
