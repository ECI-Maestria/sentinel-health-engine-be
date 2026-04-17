CREATE TABLE patient_caretakers (
    patient_id   UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    caretaker_id UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    linked_by    UUID        NOT NULL REFERENCES users(id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (patient_id, caretaker_id)
);

CREATE INDEX idx_pc_patient_id   ON patient_caretakers (patient_id);
CREATE INDEX idx_pc_caretaker_id ON patient_caretakers (caretaker_id);
