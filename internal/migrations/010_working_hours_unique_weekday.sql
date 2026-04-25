-- Migration 010: UNIQUE(barber_id, weekday) em working_hours
--
-- ANTES DE RODAR: verificar duplicatas com:
--   SELECT barber_id, weekday, COUNT(*), array_agg(id ORDER BY id)
--   FROM working_hours
--   GROUP BY barber_id, weekday
--   HAVING COUNT(*) > 1;
--
-- Esta migration mantém o registro de MAIOR id por (barber_id, weekday),
-- que representa a configuração mais recente salva via UI.
-- O handler de working_hours já usa DELETE+INSERT, portanto a constraint
-- não afeta o comportamento normal após esta migration.

BEGIN;

-- 1. Remove duplicatas: mantém o maior id (mais recente) por (barber_id, weekday).
DELETE FROM working_hours
WHERE id NOT IN (
  SELECT MAX(id)
  FROM working_hours
  GROUP BY barber_id, weekday
);

-- 2. Adiciona constraint UNIQUE.
ALTER TABLE working_hours
  ADD CONSTRAINT uq_working_hours_barber_weekday UNIQUE (barber_id, weekday);

COMMIT;
