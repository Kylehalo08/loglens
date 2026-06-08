CREATE TABLE IF NOT EXISTS services (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    metadata    JSONB NOT NULL DEFAULT '{}',
    created_by  UUID NOT NULL REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_services_org_name_active
    ON services(org_id, name) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_services_org_id ON services(org_id);
CREATE INDEX IF NOT EXISTS idx_services_org_active
    ON services(org_id) WHERE deleted_at IS NULL;
