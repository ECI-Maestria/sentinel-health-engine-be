CREATE TABLE medications (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id      UUID NOT NULL,
    prescribed_by   UUID NOT NULL,
    name            VARCHAR(255) NOT NULL,
    dosage          VARCHAR(100) NOT NULL,
    frequency       VARCHAR(30) NOT NULL,
    scheduled_times JSONB NOT NULL DEFAULT '[]',
    start_date      DATE NOT NULL,
    end_date        DATE,
    notes           TEXT,
    is_active       BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_med_patient ON medications(patient_id);
CREATE INDEX idx_med_active ON medications(patient_id, is_active);
