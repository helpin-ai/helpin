ALTER TABLE ai_connections
    ADD COLUMN IF NOT EXISTS funding text NOT NULL DEFAULT 'customer';

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ai_connections_funding_check') THEN
        ALTER TABLE ai_connections ADD CONSTRAINT ai_connections_funding_check
            CHECK (funding = 'customer' OR
                (funding = 'managed' AND scope = 'workspace' AND user_id IS NULL AND provider <> 'openai_chatgpt'));
    END IF;
END $$;
