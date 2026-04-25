-- Migration 009: UNIQUE parcial em clients(barbershop_id, phone)
--
-- ANTES DE RODAR: verificar duplicatas com:
--   SELECT barbershop_id, phone, COUNT(*), array_agg(id ORDER BY id)
--   FROM clients
--   WHERE phone IS NOT NULL AND phone <> ''
--   GROUP BY barbershop_id, phone
--   HAVING COUNT(*) > 1;
--
-- Esta migration mantém o cliente de MENOR id (o mais antigo, mais referenciado
-- em appointments/subscriptions) e remove os duplicados mais recentes.

BEGIN;

-- 1. Remove duplicatas: mantém o menor id por (barbershop_id, phone).
DELETE FROM clients
WHERE id NOT IN (
  SELECT MIN(id)
  FROM clients
  WHERE phone IS NOT NULL AND phone <> ''
  GROUP BY barbershop_id, phone
)
AND phone IS NOT NULL AND phone <> '';

-- 2. Índice único parcial: múltiplos clientes sem telefone continuam permitidos.
CREATE UNIQUE INDEX IF NOT EXISTS uq_clients_barbershop_phone
  ON clients(barbershop_id, phone)
  WHERE phone IS NOT NULL AND phone <> '';

COMMIT;
