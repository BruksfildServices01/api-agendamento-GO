-- Hot path: ListAppointmentsForDay e AssertNoTimeConflict filtram por
-- (barbershop_id, barber_id, start_time range) com status NOT IN cancelled/no_show.
-- O índice partial cobre exatamente esse filtro, reduzindo o scan nas queries
-- de disponibilidade e criação de agendamento.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_appointments_barbershop_barber_start
  ON appointments(barbershop_id, barber_id, start_time)
  WHERE status NOT IN ('cancelled', 'no_show');
