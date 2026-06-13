import type { SupportEmailSender, SupportEmailSenderDomain } from '@/lib/pmTypes';

export function buildSupportSenderEmailPreview({
  senderEmail,
  workspaceName,
  agentName,
  replyDomain,
}: {
  senderEmail: string;
  workspaceName?: string;
  agentName?: string;
  replyDomain?: string;
}) {
  const resolvedAgentName = agentName?.trim() || 'Agent';
  const resolvedWorkspaceName = workspaceName?.trim() || 'Workspace';
  const resolvedReplyDomain = replyDomain?.trim() || 'replies.helpin.email';

  return {
    from: `${resolvedAgentName} - ${resolvedWorkspaceName} <${senderEmail.trim()}>`,
    replyTo: `conv-{conversation_id}@${resolvedReplyDomain}`,
  };
}

export function groupSupportEmailSendersByDomain<
  TDomain extends Pick<SupportEmailSenderDomain, 'id' | 'domain' | 'status' | 'active'>,
  TSender extends Pick<SupportEmailSender, 'id' | 'email' | 'domain'>,
>(domains: TDomain[], senders: TSender[]): Array<{ domain: string; senderDomain: TDomain | null; senders: TSender[] }> {
  const groups = new Map<string, { domain: string; senderDomain: TDomain | null; senders: TSender[] }>();

  for (const senderDomain of domains) {
    const domain = senderDomain.domain.trim().toLowerCase();
    if (!domain) continue;
    groups.set(domain, { domain, senderDomain, senders: [] });
  }

  for (const sender of senders) {
    const domain = (sender.domain || sender.email.split('@').at(-1) || '').trim().toLowerCase();
    if (!domain) continue;
    const group = groups.get(domain) ?? { domain, senderDomain: null, senders: [] };
    group.senders.push(sender);
    groups.set(domain, group);
  }

  return Array.from(groups.values())
    .map((group) => ({
      ...group,
      senders: [...group.senders].sort((a, b) => a.email.localeCompare(b.email)),
    }))
    .sort((a, b) => {
      if (a.senderDomain && !b.senderDomain) return -1;
      if (!a.senderDomain && b.senderDomain) return 1;
      return a.domain.localeCompare(b.domain);
    });
}

export function deriveSendingDomainDefaults(workspaceName?: string, websiteUrl?: string) {
  const name = workspaceName?.trim() || 'Workspace';
  const domain = sendingDomainFromWebsiteUrl(websiteUrl) || `${slugifyWorkspaceName(name) || 'example'}.com`;

  return {
    domain,
    localPart: 'support',
    displayName: `${name} Support`,
    primaryExample: `support@${domain}`,
    secondaryExample: `billing@${domain}`,
  };
}

function sendingDomainFromWebsiteUrl(value?: string) {
  const domain = domainFromWebsiteUrl(value);
  if (!domain) return '';
  const parts = domain.split('.').filter(Boolean);
  if (parts.length < 3) return domain;

  const [firstLabel] = parts;
  const serviceSubdomains = new Set(['app', 'docs', 'help', 'inbox', 'support', 'www']);
  if (!serviceSubdomains.has(firstLabel)) return domain;
  return parts.slice(1).join('.');
}

function domainFromWebsiteUrl(value?: string) {
  const trimmed = value?.trim();
  if (!trimmed) return '';

  try {
    const url = new URL(trimmed.includes('://') ? trimmed : `https://${trimmed}`);
    return url.hostname.replace(/^www\./i, '').toLowerCase();
  } catch {
    return trimmed
      .replace(/^https?:\/\//i, '')
      .replace(/^www\./i, '')
      .split('/')[0]
      .trim()
      .toLowerCase();
  }
}

function slugifyWorkspaceName(value: string) {
  return value
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '')
    .slice(0, 40);
}
