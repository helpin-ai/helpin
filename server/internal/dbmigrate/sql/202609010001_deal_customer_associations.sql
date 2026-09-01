-- Establish one canonical customer for every deterministically resolvable deal.
-- Ambiguous legacy rows are intentionally left unlabeled for repair in the UI.

-- Canonicalize Deal <-> Contact/Company direction and keep the oldest duplicate.
WITH normalized AS (
    SELECT id,
           ROW_NUMBER() OVER (
               PARTITION BY workspace_id,
                            CASE WHEN from_object_type = 'deal' THEN from_object_id ELSE to_object_id END,
                            CASE WHEN from_object_type = 'deal' THEN to_object_type ELSE from_object_type END,
                            CASE WHEN from_object_type = 'deal' THEN to_object_id ELSE from_object_id END
               ORDER BY created_at ASC, id ASC
           ) AS row_number
    FROM crm_associations
    WHERE (from_object_type = 'deal' AND to_object_type IN ('contact', 'company'))
       OR (to_object_type = 'deal' AND from_object_type IN ('contact', 'company'))
)
DELETE FROM crm_associations
WHERE id IN (SELECT id FROM normalized WHERE row_number > 1);

UPDATE crm_associations
SET from_object_type = 'deal',
    from_object_id = to_object_id,
    to_object_type = from_object_type,
    to_object_id = from_object_id
WHERE to_object_type = 'deal'
  AND from_object_type IN ('contact', 'company');

-- A single directly linked company is deterministic.
WITH single_company AS (
    SELECT workspace_id, from_object_id AS deal_id, MIN(id::text)::uuid AS association_id
    FROM crm_associations
    WHERE from_object_type = 'deal' AND to_object_type = 'company'
    GROUP BY workspace_id, from_object_id
    HAVING COUNT(*) = 1
)
UPDATE crm_associations association
SET association_label = 'deal_customer'
FROM single_company candidate
WHERE association.id = candidate.association_id
  AND NOT EXISTS (
      SELECT 1 FROM crm_associations existing
      WHERE existing.workspace_id = candidate.workspace_id
        AND existing.from_object_type = 'deal'
        AND existing.from_object_id = candidate.deal_id
        AND existing.association_label = 'deal_customer'
  );

-- If every linked contact resolves to the same primary company, materialize it.
WITH deal_contacts AS (
    SELECT workspace_id, from_object_id AS deal_id, to_object_id AS contact_id
    FROM crm_associations
    WHERE from_object_type = 'deal' AND to_object_type = 'contact'
), resolved AS (
    SELECT dc.workspace_id, dc.deal_id, dc.contact_id,
           CASE WHEN cca.from_object_type = 'company' THEN cca.from_object_id ELSE cca.to_object_id END AS company_id
    FROM deal_contacts dc
    JOIN crm_associations cca
      ON cca.workspace_id = dc.workspace_id
     AND cca.association_label = 'primary'
     AND ((cca.from_object_type = 'contact' AND cca.from_object_id = dc.contact_id AND cca.to_object_type = 'company')
       OR (cca.to_object_type = 'contact' AND cca.to_object_id = dc.contact_id AND cca.from_object_type = 'company'))
), unanimous AS (
    SELECT dc.workspace_id, dc.deal_id, MIN(resolved.company_id::text)::uuid AS company_id
    FROM deal_contacts dc
    LEFT JOIN resolved
      ON resolved.workspace_id = dc.workspace_id
     AND resolved.deal_id = dc.deal_id
     AND resolved.contact_id = dc.contact_id
    WHERE NOT EXISTS (
        SELECT 1 FROM crm_associations direct_company
        WHERE direct_company.workspace_id = dc.workspace_id
          AND direct_company.from_object_type = 'deal'
          AND direct_company.from_object_id = dc.deal_id
          AND direct_company.to_object_type = 'company'
    )
    GROUP BY dc.workspace_id, dc.deal_id
    HAVING COUNT(*) = COUNT(resolved.company_id) AND COUNT(DISTINCT resolved.company_id) = 1
)
INSERT INTO crm_associations (workspace_id, from_object_type, from_object_id, to_object_type, to_object_id, association_label)
SELECT workspace_id, 'deal', deal_id, 'company', company_id, 'deal_customer'
FROM unanimous
ON CONFLICT DO NOTHING;

-- A lone independent contact is deterministic when no company customer exists.
WITH single_contact AS (
    SELECT workspace_id, from_object_id AS deal_id, MIN(id::text)::uuid AS association_id
    FROM crm_associations
    WHERE from_object_type = 'deal' AND to_object_type = 'contact'
    GROUP BY workspace_id, from_object_id
    HAVING COUNT(*) = 1
)
UPDATE crm_associations association
SET association_label = 'deal_customer'
FROM single_contact candidate
WHERE association.id = candidate.association_id
  AND NOT EXISTS (
      SELECT 1 FROM crm_associations customer
      WHERE customer.workspace_id = candidate.workspace_id
        AND customer.from_object_type = 'deal'
        AND customer.from_object_id = candidate.deal_id
        AND customer.association_label = 'deal_customer'
  );

-- A lone contact on a company-backed deal is its primary contact.
WITH single_contact AS (
    SELECT workspace_id, from_object_id AS deal_id, MIN(id::text)::uuid AS association_id
    FROM crm_associations
    WHERE from_object_type = 'deal' AND to_object_type = 'contact'
    GROUP BY workspace_id, from_object_id
    HAVING COUNT(*) = 1
)
UPDATE crm_associations association
SET association_label = 'deal_primary_contact'
FROM single_contact candidate
WHERE association.id = candidate.association_id
  AND EXISTS (
      SELECT 1 FROM crm_associations customer
      WHERE customer.workspace_id = candidate.workspace_id
        AND customer.from_object_type = 'deal'
        AND customer.from_object_id = candidate.deal_id
        AND customer.to_object_type = 'company'
        AND customer.association_label = 'deal_customer'
  );

-- Retain the oldest label if pre-existing data somehow had more than one.
WITH duplicates AS (
    SELECT id,
           ROW_NUMBER() OVER (
               PARTITION BY workspace_id, from_object_id, association_label
               ORDER BY created_at ASC, id ASC
           ) AS row_number
    FROM crm_associations
    WHERE from_object_type = 'deal'
      AND association_label IN ('deal_customer', 'deal_primary_contact')
)
UPDATE crm_associations
SET association_label = NULL
WHERE id IN (SELECT id FROM duplicates WHERE row_number > 1);

CREATE UNIQUE INDEX IF NOT EXISTS idx_crm_associations_one_deal_customer
    ON crm_associations (workspace_id, from_object_id)
    WHERE from_object_type = 'deal' AND association_label = 'deal_customer';

CREATE UNIQUE INDEX IF NOT EXISTS idx_crm_associations_one_deal_primary_contact
    ON crm_associations (workspace_id, from_object_id)
    WHERE from_object_type = 'deal' AND association_label = 'deal_primary_contact';
