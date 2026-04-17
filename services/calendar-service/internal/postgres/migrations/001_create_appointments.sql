CREATE TABLE appointments (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id       UUID NOT NULL,
    doctor_id        UUID NOT NULL,
    title            VARCHAR(255) NOT NULL,
    scheduled_at     TIMESTAMPTZ NOT NULL,
    location         TEXT,
    notes            TEXT,
    status           VARCHAR(20) NOT NULL DEFAULT 'SCHEDULED',
    reminder_sent_at TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_appt_patient ON appointments(patient_id);
CREATE INDEX idx_appt_scheduled ON appointments(scheduled_at);
