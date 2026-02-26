CREATE TABLE notifications (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    title         VARCHAR(255) NOT NULL,
    message       TEXT         NOT NULL,
    severity      VARCHAR(20)  NOT NULL DEFAULT 'info',
    event_type    VARCHAR(64)  NOT NULL DEFAULT '',
    resource_type VARCHAR(64)  NOT NULL DEFAULT '',
    resource_id   VARCHAR(255) NOT NULL DEFAULT '',
    read_at       TIMESTAMPTZ  NULL,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX notifications_read_at_idx ON notifications (read_at);
CREATE INDEX notifications_created_idx ON notifications (created_at DESC);

CREATE TABLE notification_channel_config (
    event_type VARCHAR(64) NOT NULL,
    channel    VARCHAR(20) NOT NULL,
    enabled    BOOLEAN     NOT NULL DEFAULT true,
    PRIMARY KEY (event_type, channel)
);

INSERT INTO notification_channel_config (event_type, channel, enabled) VALUES
  ('ssl.expiring_soon',         'in_app',  true),
  ('ssl.expiring_soon',         'email',   true),
  ('ssl.expiring_soon',         'slack',   true),
  ('ssl.expiring_soon',         'discord', true),
  ('ssl.renewed',               'in_app',  true),
  ('ssl.renewed',               'email',   true),
  ('ssl.renewed',               'slack',   true),
  ('ssl.renewed',               'discord', true),
  ('ssl.renewal_failed',        'in_app',  true),
  ('ssl.renewal_failed',        'email',   true),
  ('ssl.renewal_failed',        'slack',   true),
  ('ssl.renewal_failed',        'discord', true),
  ('domain.added',              'in_app',  true),
  ('domain.added',              'email',   false),
  ('domain.added',              'slack',   true),
  ('domain.added',              'discord', true),
  ('domain.deleted',            'in_app',  true),
  ('domain.deleted',            'email',   false),
  ('domain.deleted',            'slack',   true),
  ('domain.deleted',            'discord', true),
  ('wordpress.update_available','in_app',  true),
  ('wordpress.update_available','email',   false),
  ('wordpress.update_available','slack',   false),
  ('wordpress.update_available','discord', false),
  ('auth.login',                'in_app',  false),
  ('auth.login',                'email',   false),
  ('auth.login',                'slack',   false),
  ('auth.login',                'discord', false);
