-- SSL certificates managed by WebHostManager (one cert per domain).
CREATE TABLE ssl_certs (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id       UUID         NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    domain_name     VARCHAR(255) NOT NULL,
    status          VARCHAR(20)  NOT NULL DEFAULT 'pending',
    cert_path       VARCHAR(512) NOT NULL DEFAULT '',
    key_path        VARCHAR(512) NOT NULL DEFAULT '',
    is_wildcard     BOOLEAN      NOT NULL DEFAULT false,
    redirect_https  BOOLEAN      NOT NULL DEFAULT false,
    issued_at       TIMESTAMPTZ,
    expires_at      TIMESTAMPTZ,
    last_renewed_at TIMESTAMPTZ,
    last_error      TEXT         NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- One cert record per domain
CREATE UNIQUE INDEX ssl_certs_domain_id_uidx ON ssl_certs (domain_id);
CREATE INDEX ssl_certs_status_idx            ON ssl_certs (status);
CREATE INDEX ssl_certs_expires_at_idx        ON ssl_certs (expires_at);
