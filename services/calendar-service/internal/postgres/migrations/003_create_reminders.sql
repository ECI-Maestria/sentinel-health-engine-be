CREATE TABLE reminders (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id  UUID NOT NULL,
    created_by  UUID NOT NULL,
    title       VARCHAR(255) NOT NULL,
    message     TEXT NOT NULL,
    reminder_at TIMESTAMPTZ NOT NULL,
    recurrence  VARCHAR(20) NOT NULL DEFAULT 'NONE',
    status      VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    sent_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_rem_patient ON reminders(patient_id);
CREATE INDEX idx_rem_pending ON reminders(status, reminder_at) WHERE status = 'PENDING';
