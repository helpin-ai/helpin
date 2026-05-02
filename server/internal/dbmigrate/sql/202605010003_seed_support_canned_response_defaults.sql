WITH defaults(short_code, content, tag) AS (
  VALUES
    ('!hello', $body$Hi {{customer.first_name | fallback: "there"}}, thanks for reaching out. I'm checking this now and will get back to you shortly.$body$, 'General'),
    ('!thanks', $body$Thanks for sending this over - that helps 👍$body$, 'General'),
    ('!closing', $body$I'm going to close this for now, but reply here anytime if you need more help.$body$, 'General'),
    ('!needinfo', $body$Could you send a little more detail about what you're seeing? A screenshot, error message, or the steps you took would help us investigate.$body$, 'Support'),
    ('!steps', $body$Could you try these steps and let me know what happens?

1. Refresh the page
2. Sign out and back in
3. Try again$body$, 'Support'),
    ('!bug', $body$Thanks for reporting this. I can reproduce the issue from your description and I'm sharing it with our team to investigate.$body$, 'Support'),
    ('!escalate', $body$I'm going to escalate this to the right teammate so we can take a closer look. We'll keep you updated here.$body$, 'Support'),
    ('!invoice', $body$I can help with that. Could you confirm the billing email or invoice number so I can look it up?$body$, 'Billing'),
    ('!refund', $body$I can check the refund status for you. Please send the order ID or billing email, and I'll take a look.$body$, 'Billing'),
    ('!pricing', $body$Happy to help with pricing. Could you share your team size and what you're looking to use the product for?$body$, 'Sales'),
    ('!demo', $body$We'd be happy to walk you through it. What day and time works best for a quick demo?$body$, 'Sales'),
    ('!followup', $body$Hi {{customer.first_name | fallback: "there"}}, just checking in to see if you had a chance to review my last message.$body$, 'Follow-up')
),
empty_workspaces AS (
  SELECT w.id
  FROM workspaces w
  WHERE NOT EXISTS (
    SELECT 1
    FROM support_canned_responses r
    WHERE r.workspace_id = w.id
  )
)
INSERT INTO support_canned_responses (
  workspace_id,
  short_code,
  content,
  tag,
  created_at,
  updated_at
)
SELECT
  empty_workspaces.id,
  defaults.short_code,
  defaults.content,
  defaults.tag,
  NOW(),
  NOW()
FROM empty_workspaces
CROSS JOIN defaults
ON CONFLICT DO NOTHING;
