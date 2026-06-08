CREATE TABLE IF NOT EXISTS service_api_keys (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id      UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    prefix          VARCHAR(16) NOT NULL UNIQUE,
    key_hash        VARCHAR(255) NOT NULL,
    label           VARCHAR(100),
    created_by      UUID NOT NULL REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at      TIMESTAMPTZ,
    last_used_at    TIMESTAMPTZ,
    rotated_from_id UUID REFERENCES service_api_keys(id),

    CONSTRAINT chk_revoked_after_created
        CHECK (revoked_at IS NULL OR revoked_at >= created_at)
);

CREATE INDEX IF NOT EXISTS idx_api_keys_service_id
    ON service_api_keys(service_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_active_prefix
    ON service_api_keys(prefix) WHERE revoked_at IS NULL;
