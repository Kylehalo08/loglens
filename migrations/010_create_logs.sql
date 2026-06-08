CREATE TABLE IF NOT EXISTS logs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    service_id  UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    timestamp   TIMESTAMPTZ NOT NULL,
    severity    VARCHAR(10) NOT NULL CHECK (severity IN ('DEBUG','INFO','WARN','ERROR','FATAL')),
    message     TEXT NOT NULL,
    metadata    JSONB NOT NULL DEFAULT '{}',
    ingested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    message_tsv tsvector GENERATED ALWAYS AS (to_tsvector('english', coalesce(message, ''))) STORED
);

CREATE INDEX IF NOT EXISTS idx_logs_service_time ON logs (service_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_logs_org_time ON logs (org_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_logs_service_severity_time ON logs (service_id, severity, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_logs_message_fts ON logs USING GIN (message_tsv);
CREATE INDEX IF NOT EXISTS idx_logs_timestamp ON logs (timestamp);
