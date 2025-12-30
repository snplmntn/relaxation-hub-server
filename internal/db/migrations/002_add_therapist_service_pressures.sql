-- Migration: add therapist_service_pressures table
CREATE TABLE IF NOT EXISTS therapist_service_pressures (
    therapist_id INT NOT NULL REFERENCES therapist_profiles(therapist_id) ON DELETE CASCADE,
    service_id INT NOT NULL REFERENCES services(service_id) ON DELETE CASCADE,
    pressure VARCHAR(10) NOT NULL CHECK (pressure IN ('soft','medium','hard')),
    PRIMARY KEY (therapist_id, service_id, pressure)
);

CREATE INDEX IF NOT EXISTS idx_tsp_service ON therapist_service_pressures(service_id);
CREATE INDEX IF NOT EXISTS idx_tsp_therapist ON therapist_service_pressures(therapist_id);
