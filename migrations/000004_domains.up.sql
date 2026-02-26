-- Hosted domains managed by WebHostManager.
CREATE TABLE domains (
    id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name         VARCHAR(255) NOT NULL UNIQUE,
    status       VARCHAR(20)  NOT NULL DEFAULT 'active',
    disk_used_mb BIGINT       NOT NULL DEFAULT 0,
    web_root     VARCHAR(512) NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX domains_status_idx ON domains (status);
CREATE INDEX domains_name_idx   ON domains (name);
