CREATE TABLE IF NOT EXISTS audit_events (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        UUID REFERENCES organizations(id),
    actor_id      UUID REFERENCES users(id),
    actor_type    VARCHAR(20) NOT NULL DEFAULT 'user',
    action        VARCHAR(50) NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    resource_id   UUID,
    metadata      JSONB NOT NULL DEFAULT '{}',
    ip_address    INET,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_audit_org_created
    ON audit_events(org_id, created_at DESC);
