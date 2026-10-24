import type { SupportEmailRoute, SupportEmailSender } from '@/lib/pmTypes';

export type InboxSenderAddress = {
  sender: SupportEmailSender;
  // 'workspace' means the inbox has no sender of its own and replies use the workspace default.
  source: 'inbox' | 'workspace';
};

// Matches the Team inboxes settings table: only active routes bound to this inbox.
export function getInboxForwardingRoutes(routes: SupportEmailRoute[], mailboxId?: string | null) {
  if (!mailboxId) return [];
  return routes.filter((route) => route.active && route.mailbox_id === mailboxId);
}

export function getInboxSenderAddress(senders: SupportEmailSender[], mailboxId?: string | null): InboxSenderAddress | null {
  const activeSenders = senders.filter((sender) => sender.active);
  const inboxSender = mailboxId
    ? activeSenders.find((sender) =>
        sender.default_scope === 'mailbox' &&
        (sender.mailbox_id === mailboxId || (sender.mailbox_ids ?? []).includes(mailboxId)),
      )
    : undefined;
  if (inboxSender) return { sender: inboxSender, source: 'inbox' };
  const workspaceSender = activeSenders.find((sender) => sender.default_scope === 'workspace');
  return workspaceSender ? { sender: workspaceSender, source: 'workspace' } : null;
}

// Same rule the sender picker uses to decide a sender can send mail.
export function isSenderVerified(sender: Pick<SupportEmailSender, 'dkim_verified' | 'return_path_domain_verified'>) {
  return sender.dkim_verified && sender.return_path_domain_verified;
}
