CREATE TABLE devices (
    id                UUID         PRIMARY KEY,
    user_id           UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_identifier VARCHAR(255) NOT NULL,
    fcm_token         TEXT,
    platform          VARCHAR(10)  NOT NULL CHECK (platform IN ('ANDROID', 'IOS')),
    name              VARCHAR(255),
    is_active         BOOLEAN      NOT NULL DEFAULT true,
    last_seen_at      TIMESTAMPTZ,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, device_identifier)
);

CREATE INDEX idx_devices_user_id           ON devices (user_id);
CREATE INDEX idx_devices_device_identifier ON devices (device_identifier);
