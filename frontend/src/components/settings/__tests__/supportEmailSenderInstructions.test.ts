import { describe, expect, it } from 'vitest';
import { buildSenderInstructions, senderDNSRecords } from '../supportEmailSenderInstructions';

const details = {
  domain: 'example.com', dkim_host: 'old._domainkey', dkim_text_value: 'old-key', dkim_verified: true,
  dkim_pending_host: 'new._domainkey', dkim_pending_text_value: `k=rsa; p=${'AB+/'.repeat(150)}`,
  return_path_domain: 'pm-bounces', return_path_domain_cname_value: 'pm.example.net', return_path_domain_verified: false,
};
describe('sender DNS instructions', () => {
  it('shows the pending key without marking it verified based on the old key', () => {
    const records = senderDNSRecords(details);
    expect(records[0]).toMatchObject({ name: 'new._domainkey', value: details.dkim_pending_text_value, verified: false });
    expect(records[1]).toMatchObject({ type: 'CNAME', name: 'pm-bounces', verified: false });
  });
  it.each(['ai', 'developer'] as const)('copies complete displayed records and the real settings link for %s', audience => {
    const text = buildSenderInstructions(audience, 'hello@example.com', details, 'https://selfhost.example.test/w/example/settings/inboxes-routing?tab=senders');
    for (const record of senderDNSRecords(details)) {
      expect(text).toContain(`Type: ${record.type}`); expect(text).toContain(`Name: ${record.name}`); expect(text).toContain(`Value: ${record.value}`);
    }
    expect(text).toContain('https://selfhost.example.test/');
    expect(text).toContain('MX, SPF, or DMARC');
    expect(text).not.toContain('MOCKUP');
  });
});
