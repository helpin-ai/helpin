-- When the customer last opened each portal request, for new-reply markers.
ALTER TABLE support_portal_request_references ADD COLUMN IF NOT EXISTS customer_last_read_at TIMESTAMPTZ;
