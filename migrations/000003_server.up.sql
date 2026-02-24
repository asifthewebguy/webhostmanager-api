-- Server connection configuration (single row).
CREATE TABLE server_config (
    id                    INTEGER PRIMARY KEY DEFAULT 1,
    connection_mode       VARCHAR(10)  NOT NULL DEFAULT 'local',
    ssh_host              VARCHAR(255) NOT NULL DEFAULT '',
    ssh_port              INTEGER      NOT NULL DEFAULT 22,
    ssh_user              VARCHAR(100) NOT NULL DEFAULT '',
    ssh_auth_type         VARCHAR(20)  NOT NULL DEFAULT 'key',
    ssh_key_encrypted     TEXT         NOT NULL DEFAULT '',
    ssh_password_encrypted TEXT        NOT NULL DEFAULT '',
    updated_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT server_config_single_row CHECK (id = 1)
);

INSERT INTO server_config (id, connection_mode)
VALUES (1, 'local')
ON CONFLICT (id) DO NOTHING;

-- Metrics cache: stores the last collected server performance snapshot.
CREATE TABLE metrics_cache (
    metric_name  VARCHAR(50) PRIMARY KEY,
    value        TEXT        NOT NULL DEFAULT '',
    collected_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
