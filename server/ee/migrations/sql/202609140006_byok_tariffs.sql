CREATE TABLE IF NOT EXISTS ai_byok_tariffs (
    version text PRIMARY KEY CHECK (btrim(version) <> ''),
    currency text NOT NULL CHECK (currency = 'USD'),
    microusd_per_million bigint NOT NULL CHECK (microusd_per_million >= 0),
    accounting_version text NOT NULL CHECK (accounting_version = 'normalized-tokens-v1')
);

CREATE OR REPLACE FUNCTION ai_byok_tariff_immutable() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' OR NEW IS DISTINCT FROM OLD THEN
        RAISE EXCEPTION 'BYOK tariffs are immutable; insert a new version';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS ai_byok_tariff_immutable ON ai_byok_tariffs;
CREATE TRIGGER ai_byok_tariff_immutable BEFORE UPDATE OR DELETE ON ai_byok_tariffs
    FOR EACH ROW EXECUTE FUNCTION ai_byok_tariff_immutable();

CREATE TABLE IF NOT EXISTS workspace_ai_billing_settings (
    workspace_id uuid PRIMARY KEY REFERENCES workspaces(id) ON DELETE CASCADE,
    byok_enabled boolean NOT NULL DEFAULT false,
    tariff_version text REFERENCES ai_byok_tariffs(version),
    CHECK (NOT byok_enabled OR tariff_version IS NOT NULL)
);
