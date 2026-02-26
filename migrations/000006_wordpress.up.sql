CREATE TABLE wordpress_installs (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id   UUID         NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    domain_name VARCHAR(255) NOT NULL,
    db_name     VARCHAR(128) NOT NULL DEFAULT '',
    db_user     VARCHAR(128) NOT NULL DEFAULT '',
    db_password TEXT         NOT NULL DEFAULT '',
    wp_version  VARCHAR(20)  NOT NULL DEFAULT '',
    wp_url      VARCHAR(512) NOT NULL DEFAULT '',
    admin_user  VARCHAR(128) NOT NULL DEFAULT '',
    admin_email VARCHAR(255) NOT NULL DEFAULT '',
    debug_mode  BOOLEAN      NOT NULL DEFAULT false,
    status      VARCHAR(20)  NOT NULL DEFAULT 'installed',
    last_error  TEXT         NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX wordpress_installs_domain_id_uidx ON wordpress_installs (domain_id);
