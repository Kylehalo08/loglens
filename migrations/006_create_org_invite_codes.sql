CREATE TABLE IF NOT EXISTS org_invite_codes (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    code          VARCHAR(20) NOT NULL UNIQUE,
    created_by    UUID NOT NULL REFERENCES users(id),
    default_role  VARCHAR(20) NOT NULL DEFAULT 'developer'
                  CHECK (default_role IN ('admin','developer','viewer')),
    is_active     BOOLEAN NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_org_invite_codes_code ON org_invite_codes(code);
