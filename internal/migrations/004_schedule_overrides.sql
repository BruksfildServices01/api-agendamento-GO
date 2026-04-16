-- Exceções de horário por data específica ou por dia da semana em um mês.
-- Hierarquia de prioridade: data específica > dia da semana no mês > horário padrão semanal.
CREATE TABLE schedule_overrides (
  id            BIGSERIAL   PRIMARY KEY,
  barbershop_id BIGINT      NOT NULL REFERENCES barbershops(id) ON DELETE CASCADE,
  barber_id     BIGINT      NOT NULL REFERENCES users(id)       ON DELETE CASCADE,

  -- escopo: date XOR (weekday + month + year)
  date          DATE,
  weekday       SMALLINT,
  month         SMALLINT,
  year          SMALLINT,

  -- comportamento
  closed        BOOLEAN     NOT NULL DEFAULT FALSE,
  start_time    VARCHAR(5),   -- HH:MM  (null = dia fechado)
  end_time      VARCHAR(5),   -- HH:MM  (null = dia fechado)

  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  CONSTRAINT scope_check CHECK (
    (date IS NOT NULL AND weekday IS NULL AND month IS NULL AND year IS NULL)
    OR
    (date IS NULL AND weekday IS NOT NULL AND month IS NOT NULL AND year IS NOT NULL)
  )
);

-- Unicidade: um barbeiro só pode ter uma exceção por data específica
CREATE UNIQUE INDEX schedule_overrides_date_idx
  ON schedule_overrides (barber_id, date)
  WHERE date IS NOT NULL;

-- Unicidade: um barbeiro só pode ter uma exceção por combinação dia-da-semana/mês/ano
CREATE UNIQUE INDEX schedule_overrides_weekday_idx
  ON schedule_overrides (barber_id, weekday, month, year)
  WHERE weekday IS NOT NULL;
