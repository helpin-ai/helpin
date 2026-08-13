UPDATE support_conversations AS conversation
SET customer_name = BTRIM(CONCAT_WS(' ', contact.first_name, contact.last_name)),
    customer_email = contact.email,
    customer_phone = contact.phone
FROM crm_contacts AS contact
WHERE conversation.workspace_id = contact.workspace_id
  AND conversation.crm_contact_id = contact.id
  AND (
    conversation.customer_name IS DISTINCT FROM BTRIM(CONCAT_WS(' ', contact.first_name, contact.last_name))
    OR conversation.customer_email IS DISTINCT FROM contact.email
    OR conversation.customer_phone IS DISTINCT FROM contact.phone
  );
