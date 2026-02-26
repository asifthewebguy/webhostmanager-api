CREATE TABLE email_accounts (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id   UUID         NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    domain_name VARCHAR(255) NOT NULL,
    username    VARCHAR(64)  NOT NULL,
    email       VARCHAR(255) NOT NULL,
    password    TEXT         NOT NULL DEFAULT '',
    quota_mb    INT          NOT NULL DEFAULT 500,
    status      VARCHAR(20)  NOT NULL DEFAULT 'active',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX email_accounts_email_uidx ON email_accounts (email);

CREATE TABLE email_forwarders (
    id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id    UUID         NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    domain_name  VARCHAR(255) NOT NULL,
    source       VARCHAR(255) NOT NULL,
    destination  TEXT         NOT NULL,
    is_catch_all BOOLEAN      NOT NULL DEFAULT false,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX email_forwarders_source_uidx ON email_forwarders (source);
