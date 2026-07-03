// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

import type { SupportMessage, SupportMessageEmailDetail } from '@/lib/pmTypes'
import { EmailDetailModal } from '../EmailDetailModal'

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const mockUseMessageEmailDetail = vi.fn()

vi.mock('@/hooks/queries/useSupport', () => ({
  useMessageEmailDetail: (...args: unknown[]) => mockUseMessageEmailDetail(...args),
}))

vi.mock('@/components/ui/dialog', () => ({
  Dialog: ({ open, children }: { open: boolean; children: React.ReactNode }) => (open ? <div>{children}</div> : null),
  DialogContent: ({ children, className }: { children: React.ReactNode; className?: string }) => <section className={className}>{children}</section>,
  DialogTitle: ({ children, className }: { children: React.ReactNode; className?: string }) => <h2 className={className}>{children}</h2>,
}))

vi.mock('../EmailBodyRenderer', () => ({
  EmailBodyRenderer: ({ html, collapsedByDefault }: { html: string; collapsedByDefault?: boolean }) => (
    <div data-collapsed-by-default={String(!!collapsedByDefault)} dangerouslySetInnerHTML={{ __html: html }} />
  ),
}))

const baseMessage: SupportMessage = {
  id: 'message-1',
  workspace_id: 'workspace-1',
  conversation_id: 'conversation-1',
  sender_type: 'customer',
  sender_display_name: 'Taylor Visitor',
  content: 'fallback body',
  message_type: 'reply',
  is_internal: false,
  via_channel: 'email',
  created_at: '2026-06-02T10:14:00.000Z',
  updated_at: '2026-06-02T10:14:00.000Z',
}

function renderModal(detail: SupportMessageEmailDetail, messageOverrides: Partial<SupportMessage> = {}) {
  mockUseMessageEmailDetail.mockReturnValue({ data: detail, isLoading: false, isError: false })

  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)

  act(() => {
    root.render(
      <EmailDetailModal
        workspaceId="workspace-1"
        message={{ ...baseMessage, ...messageOverrides }}
        open
        onOpenChange={() => undefined}
      />,
    )
  })

  return {
    container,
    cleanup: () => {
      act(() => {
        root.unmount()
      })
      container.remove()
    },
  }
}

afterEach(() => {
  mockUseMessageEmailDetail.mockReset()
})

describe('EmailDetailModal', () => {
  it('shows available headers, message metadata, and full email body by default', () => {
    const rendered = renderModal({
      id: 'log-1',
      message_id: 'message-1',
      direction: 'inbound',
      subject: 'Website inquiry',
      from_email: 'Acme Contact Form <website@acme.com>',
      reply_to: 'Taylor Visitor <taylor.visitor@example.com>',
      to_email: 'Helpin Support <inbox@acme.on.helpin.email>',
      cc_emails: ['sales@acme.com'],
      bcc_emails: ['audit@acme.com'],
      rfc_message_id: '<message-1@acme.com>',
      in_reply_to: '<prior@customer.example>',
      references_header: '<root@customer.example> <prior@customer.example>',
      stripped_text: 'Visible body\n\nOn Monday, prior quote',
      html_body: '<p>Visible body</p><blockquote>Prior quoted content</blockquote>',
      status: 'sent',
      created_at: '2026-06-02T10:14:00.000Z',
    })

    expect(rendered.container.textContent).toContain('From')
    expect(rendered.container.textContent).toContain('website@acme.com')
    expect(rendered.container.textContent).toContain('Reply-To')
    expect(rendered.container.textContent).toContain('taylor.visitor@example.com')
    expect(rendered.container.textContent).toContain('Cc')
    expect(rendered.container.textContent).toContain('sales@acme.com')
    expect(rendered.container.textContent).toContain('Bcc')
    expect(rendered.container.textContent).toContain('audit@acme.com')
    expect(rendered.container.textContent).toContain('Received at')
    expect(rendered.container.textContent).not.toContain('Date')
    expect(rendered.container.textContent).not.toContain('Technical details')
    expect(rendered.container.textContent).not.toContain('Direction')
    expect(rendered.container.textContent).not.toContain('Status')
    expect(rendered.container.textContent).not.toContain('inbound')
    expect(rendered.container.textContent).not.toContain('sent')
    expect(rendered.container.textContent).toContain('Message-ID')
    expect(rendered.container.textContent).toContain('<message-1@acme.com>')
    expect(rendered.container.textContent).toContain('In-Reply-To')
    expect(rendered.container.textContent).toContain('<prior@customer.example>')
    expect(rendered.container.textContent).not.toContain('References')
    expect(rendered.container.textContent).not.toContain('<root@customer.example>')
    expect(rendered.container.querySelector('[data-testid="email-body-scroll"]')?.className).toContain('pb-8')
    expect(rendered.container.querySelector('[data-collapsed-by-default]')?.getAttribute('data-collapsed-by-default')).toBe('false')
    expect(rendered.container.innerHTML).toContain('Prior quoted content')
    expect(rendered.container.textContent).not.toContain('Show technical details')

    rendered.cleanup()
  })

  it('shows image attachments as thumbnails with preview navigation in full email dialog', () => {
    const rendered = renderModal(
      {
        id: 'log-attachments',
        message_id: 'message-attachments',
        direction: 'inbound',
        subject: 'Screenshots attached',
        from_email: 'Taylor <taylor@example.com>',
        to_email: 'Support <support@example.com>',
        stripped_text: 'See attached images.',
        html_body: '<p>See attached images.</p>',
        status: 'sent',
        created_at: '2026-06-02T10:14:00.000Z',
      },
      {
        attachments: [
          {
            id: 'img-1',
            file_key: 'support/img-1',
            file_name: 'screen-one.png',
            file_type: 'image/png',
            file_size: 2048,
            url: 'https://cdn.example.com/screen-one.png',
          },
          {
            id: 'img-2',
            file_key: 'support/img-2',
            file_name: 'screen-two.png',
            file_type: 'image/png',
            file_size: 3072,
            url: 'https://cdn.example.com/screen-two.png',
          },
          {
            id: 'pdf-1',
            file_key: 'support/pdf-1',
            file_name: 'invoice.pdf',
            file_type: 'application/pdf',
            file_size: 4096,
            url: 'https://cdn.example.com/invoice.pdf',
          },
        ],
      },
    )

    const firstThumb = rendered.container.querySelector('img[alt="screen-one.png"]') as HTMLImageElement | null
    expect(firstThumb?.className).toContain('h-full')
    expect(firstThumb?.closest('button')?.className).toContain('h-20')
    expect(rendered.container.querySelector('a[href="https://cdn.example.com/invoice.pdf"]')?.textContent).toContain('invoice.pdf')

    act(() => {
      firstThumb?.closest('button')?.dispatchEvent(new MouseEvent('mouseover', { bubbles: true }))
    })
    expect(rendered.container.querySelector('[data-testid="support-attachment-hover-preview"] img')?.getAttribute('src')).toBe('https://cdn.example.com/screen-one.png')

    act(() => {
      firstThumb?.closest('button')?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })
    expect(document.body.querySelector('[data-testid="support-attachment-lightbox"] img')?.getAttribute('src')).toBe('https://cdn.example.com/screen-one.png')

    rendered.cleanup()
  })
})
