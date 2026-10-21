import type { SupportEmailSenderDomain } from '@/lib/pmTypes';

export type SenderDNSDetails = Pick<SupportEmailSenderDomain,
  'domain' | 'dkim_host' | 'dkim_text_value' | 'dkim_pending_host' | 'dkim_pending_text_value' |
  'dkim_verified' | 'return_path_domain' | 'return_path_domain_cname_value' | 'return_path_domain_verified'>;

export function senderDNSRecords(details: SenderDNSDetails) {
  const pendingKey = Boolean(details.dkim_pending_host && details.dkim_pending_text_value &&
    (details.dkim_pending_host !== details.dkim_host || details.dkim_pending_text_value !== details.dkim_text_value));
  return [
    {
      label: 'DKIM', type: 'TXT', name: pendingKey ? details.dkim_pending_host : details.dkim_host,
      value: pendingKey ? details.dkim_pending_text_value : details.dkim_text_value,
      verified: details.dkim_verified && !pendingKey,
      description: 'Add this TXT record so mail providers can verify Helpin is allowed to send for your domain.',
    },
    {
      label: 'Return-Path', type: 'CNAME', name: details.return_path_domain,
      value: details.return_path_domain_cname_value, verified: details.return_path_domain_verified,
      description: 'Add this CNAME record so bounce handling aligns with your domain.',
    },
  ];
}

export function buildSenderInstructions(audience: 'ai' | 'developer', email: string, details: SenderDNSDetails, settingsUrl: string) {
  const records = senderDNSRecords(details).map(record =>
    `${record.label}${record.verified ? ' (already verified)' : ''}\nType: ${record.type}\nName: ${record.name || 'Unavailable — refresh records in Helpin'}\nValue: ${record.value || 'Unavailable — refresh records in Helpin'}`,
  ).join('\n\n');
  return `${audience === 'ai' ? 'Help me configure' : 'Configure'} DNS for the Helpin support sender ${email}.
Domain: ${details.domain}
Helpin settings: ${settingsUrl}

${records}

${audience === 'ai'
    ? 'Use my connected DNS provider if available; otherwise tell me how to add these records. Inspect existing records first. Skip exact matches and explain conflicting records before replacing them.'
    : 'Open the DNS settings for this domain. Add the missing records above, skipping exact matches. Resolve conflicting records before saving.'}
Copy values in full without quotes or line breaks. If the DNS provider appends the domain automatically, enter only the relative record name. Use the default TTL. On Cloudflare, set the Return-Path CNAME to DNS only.
Do not change unrelated records, including MX, SPF, or DMARC. These records authorize sending; they do not create a mailbox or configure incoming email forwarding.
Return to the Helpin settings link, open this address, and check verification. If DNS is still pending, allow propagation time and check again. If the check fails, report the error rather than treating it as verified. Do not claim the address is in use until Helpin has also saved the chosen inbox assignment.`;
}
