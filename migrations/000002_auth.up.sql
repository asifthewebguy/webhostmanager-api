CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Users
CREATE TABLE users (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    username     VARCHAR(100) NOT NULL,
    email        VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role         VARCHAR(50)  NOT NULL DEFAULT 'viewer',
    is_active    BOOLEAN      NOT NULL DEFAULT true,
    last_login_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT users_username_unique UNIQUE (username),
    CONSTRAINT users_email_unique    UNIQUE (email),
    CONSTRAINT users_role_check CHECK (role IN ('super_admin','admin','developer','viewer'))
);
CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_email    ON users(email);

-- Key-value settings store (values may be AES-256-GCM encrypted)
CREATE TABLE settings (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    key          VARCHAR(255) NOT NULL,
    value        TEXT         NOT NULL DEFAULT '',
    is_encrypted BOOLEAN      NOT NULL DEFAULT false,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT settings_key_unique UNIQUE (key)
);
CREATE INDEX idx_settings_key ON settings(key);

-- First-run wizard state (always exactly one row)
CREATE TABLE setup_state (
    id           INTEGER PRIMARY KEY DEFAULT 1,
    current_step INTEGER  NOT NULL DEFAULT 0,
    is_complete  BOOLEAN  NOT NULL DEFAULT false,
    completed_at TIMESTAMPTZ,
    CONSTRAINT setup_state_single_row CHECK (id = 1)
);
INSERT INTO setup_state (id, current_step, is_complete) VALUES (1, 0, false);

-- Audit log
CREATE TABLE audit_logs (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID        REFERENCES users(id) ON DELETE SET NULL,
    username      VARCHAR(100),
    role          VARCHAR(50),
    action        VARCHAR(255) NOT NULL,
    resource_type VARCHAR(100),
    resource_id   VARCHAR(255),
    details       JSONB,
    ip_address    VARCHAR(45),
    user_agent    TEXT,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_audit_logs_user_id    ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at);
CREATE INDEX idx_audit_logs_action     ON audit_logs(action);
