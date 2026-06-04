UPDATE support_canned_responses
SET
  content = $body$Hi {{customer.first_name | fallback: "there"}},

Thanks for reaching out. I'm checking this now and will get back to you shortly.$body$,
  updated_at = NOW()
WHERE short_code = '!hello'
  AND content = $body$Hi {{customer.first_name | fallback: "there"}}, thanks for reaching out. I'm checking this now and will get back to you shortly.$body$;

UPDATE support_canned_responses
SET
  content = $body$Could you send a little more detail about what you're seeing?

A screenshot, error message, or the steps you took would help us investigate.$body$,
  updated_at = NOW()
WHERE short_code = '!needinfo'
  AND content = $body$Could you send a little more detail about what you're seeing? A screenshot, error message, or the steps you took would help us investigate.$body$;

UPDATE support_canned_responses
SET
  content = $body$Thanks for reporting this.

I can reproduce the issue from your description and I'm sharing it with our team to investigate.$body$,
  updated_at = NOW()
WHERE short_code = '!bug'
  AND content = $body$Thanks for reporting this. I can reproduce the issue from your description and I'm sharing it with our team to investigate.$body$;

UPDATE support_canned_responses
SET
  content = $body$I'm going to escalate this to the right teammate so we can take a closer look.

We'll keep you updated here.$body$,
  updated_at = NOW()
WHERE short_code = '!escalate'
  AND content = $body$I'm going to escalate this to the right teammate so we can take a closer look. We'll keep you updated here.$body$;

UPDATE support_canned_responses
SET
  content = $body$I can help with that.

Could you confirm the billing email or invoice number so I can look it up?$body$,
  updated_at = NOW()
WHERE short_code = '!invoice'
  AND content = $body$I can help with that. Could you confirm the billing email or invoice number so I can look it up?$body$;

UPDATE support_canned_responses
SET
  content = $body$I can check the refund status for you.

Please send the order ID or billing email, and I'll take a look.$body$,
  updated_at = NOW()
WHERE short_code = '!refund'
  AND content = $body$I can check the refund status for you. Please send the order ID or billing email, and I'll take a look.$body$;

UPDATE support_canned_responses
SET
  content = $body$Happy to help with pricing.

Could you share your team size and what you're looking to use the product for?$body$,
  updated_at = NOW()
WHERE short_code = '!pricing'
  AND content = $body$Happy to help with pricing. Could you share your team size and what you're looking to use the product for?$body$;

UPDATE support_canned_responses
SET
  content = $body$Hi {{customer.first_name | fallback: "there"}},

Just checking in to see if you had a chance to review my last message.$body$,
  updated_at = NOW()
WHERE short_code = '!followup'
  AND content = $body$Hi {{customer.first_name | fallback: "there"}}, just checking in to see if you had a chance to review my last message.$body$;
