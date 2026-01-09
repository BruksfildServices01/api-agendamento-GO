CREATE EXTENSION IF NOT EXISTS btree_gist;


CREATE INDEX IF NOT EXISTS idx_appointments_barber_time
ON appointments (barber_id, start_time, end_time);

ALTER TABLE appointments
ADD CONSTRAINT no_overlapping_appointments
EXCLUDE USING GIST (
  barber_id WITH =,
  tsrange(start_time, end_time, '[)') WITH &&
)
WHERE (status = 'scheduled');
