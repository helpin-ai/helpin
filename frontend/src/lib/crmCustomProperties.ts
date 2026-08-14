const SYSTEM_CRM_CUSTOM_PROPERTY_KEYS = new Set([
  'agent_enrichment',
]);

/** Internal CRM metadata is retained for audit/provenance, but is not a customer attribute. */
export function isSystemCRMCustomProperty(key: string): boolean {
  return SYSTEM_CRM_CUSTOM_PROPERTY_KEYS.has(key);
}
