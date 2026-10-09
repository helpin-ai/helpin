-- NULL retains the existing assignment behavior until an inbox is configured.
-- Explicit pools contain workspace member IDs; [] means no automatic recipients.
ALTER TABLE support_mailboxes ADD COLUMN IF NOT EXISTS assignment_member_ids jsonb;
