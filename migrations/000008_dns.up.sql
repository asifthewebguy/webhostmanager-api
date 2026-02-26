CREATE TABLE dns_records (
    id                   UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id            UUID         NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    domain_name          VARCHAR(255) NOT NULL,
    cloudflare_zone_id   VARCHAR(64)  NOT NULL DEFAULT '',
    cloudflare_record_id VARCHAR(64)  NOT NULL DEFAULT '',
    type                 VARCHAR(10)  NOT NULL,
    name                 VARCHAR(255) NOT NULL,
    content              TEXT         NOT NULL,
    ttl                  INT          NOT NULL DEFAULT 1,
    proxied              BOOLEAN      NOT NULL DEFAULT false,
    priority             INT          NOT NULL DEFAULT 0,
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX dns_records_domain_id_idx ON dns_records (domain_id);
CREATE UNIQUE INDEX dns_records_cf_record_id_uidx ON dns_records (cloudflare_record_id) WHERE cloudflare_record_id <> '';
