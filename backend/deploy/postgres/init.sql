CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS applications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username TEXT NOT NULL UNIQUE,
    role TEXT NOT NULL CHECK (role IN ('admin', 'engineer')),
    password_hash TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS password_hash TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS user_applications (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    application_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, application_id)
);

CREATE TABLE IF NOT EXISTS alert_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID REFERENCES applications(id) ON DELETE CASCADE,
    level TEXT NOT NULL CHECK (level IN ('ERROR', 'CRITICAL')),
    enabled BOOLEAN NOT NULL DEFAULT true,
    dedup_window_seconds INTEGER NOT NULL DEFAULT 60 CHECK (dedup_window_seconds > 0),
    telegram_enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (application_id, level)
);

CREATE TABLE IF NOT EXISTS incidents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_name TEXT NOT NULL,
    fingerprint TEXT NOT NULL,
    level TEXT NOT NULL CHECK (level IN ('ERROR', 'CRITICAL')),
    category TEXT NOT NULL,
    title TEXT NOT NULL,
    first_seen_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    occurrence_count BIGINT NOT NULL DEFAULT 0,
    suppressed_count BIGINT NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN', 'ACKED', 'RESOLVED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (fingerprint)
);

CREATE INDEX IF NOT EXISTS idx_incidents_status_last_seen
    ON incidents (status, last_seen_at DESC);

CREATE INDEX IF NOT EXISTS idx_incidents_application_last_seen
    ON incidents (application_name, last_seen_at DESC);

INSERT INTO applications (name, display_name)
VALUES
    ('payment-service', 'Payment Service'),
    ('auth-service', 'Auth Service'),
    ('order-service', 'Order Service')
ON CONFLICT (name) DO NOTHING;

INSERT INTO users (username, role)
VALUES
    ('admin', 'admin'),
    ('engineer-payment', 'engineer')
ON CONFLICT (username) DO NOTHING;

INSERT INTO user_applications (user_id, application_id)
SELECT u.id, a.id
FROM users u
JOIN applications a ON a.name IN ('payment-service', 'order-service')
WHERE u.username = 'engineer-payment'
ON CONFLICT DO NOTHING;

INSERT INTO alert_rules (application_id, level, enabled, dedup_window_seconds, telegram_enabled)
SELECT a.id, levels.level, true, 60, true
FROM applications a
CROSS JOIN (VALUES ('ERROR'), ('CRITICAL')) AS levels(level)
ON CONFLICT (application_id, level) DO NOTHING;
