-- Helpin keeps a stable key for threads, pending jobs, and deduplication.
-- Each Slack ID for the channel resolves to that key and the current API ID.
CREATE TABLE IF NOT EXISTS slack_channel_ids (
    account_id uuid NOT NULL REFERENCES support_channel_accounts(id) ON DELETE CASCADE,
    channel_id text NOT NULL,
    routing_id text NOT NULL,
    current_id text NOT NULL,
    PRIMARY KEY (account_id, channel_id)
);
CREATE INDEX IF NOT EXISTS slack_channel_ids_by_route ON slack_channel_ids(account_id, routing_id);

INSERT INTO slack_channel_ids(account_id, channel_id, routing_id, current_id)
SELECT account_id, channel_id, channel_id, channel_id FROM (
    SELECT account_id, channel_id FROM slack_channel_routes
    UNION SELECT account_id, channel_id FROM slack_task_threads
    UNION SELECT channel_account_id, split_part(provider_thread_key, ':', 1)
          FROM support_conversations WHERE channel='slack' AND channel_account_id IS NOT NULL AND provider_thread_key IS NOT NULL
) existing WHERE channel_id <> ''
ON CONFLICT DO NOTHING;
