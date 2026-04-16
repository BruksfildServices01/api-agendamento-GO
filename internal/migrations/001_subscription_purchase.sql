-- =============================================================
-- Migration 001 — Subscription purchase + appointment coverage
-- =============================================================
-- Rode no Neon (SQL Editor) em dois passos:
--
-- PASSO 1: rode só o bloco fora da transação (ALTER TYPE ADD VALUE).
-- PASSO 2: rode o BEGIN…COMMIT com o restante.
--
-- Motivo: ALTER TYPE ADD VALUE não pode rodar dentro de uma
-- transação em PostgreSQL antes da versão 12. No Neon (PG 15)
-- isso já não é problema, mas separar é mais seguro e explícito.
-- =============================================================

-- ── PASSO 1 (rode isolado) ────────────────────────────────────

ALTER TYPE subscription_status ADD VALUE IF NOT EXISTS 'pending_payment';

CREATE TYPE IF NOT EXISTS coverage_status AS ENUM (
  'none',
  'covered',
  'not_covered_service',
  'not_covered_exhausted',
  'not_covered_expired'
);

-- ── PASSO 2 (rode dentro da transação) ───────────────────────

BEGIN;

-- subscriptions: campo de reserva de usos
ALTER TABLE subscriptions
  ADD COLUMN IF NOT EXISTS cuts_reserved_in_period INTEGER NOT NULL DEFAULT 0
  CONSTRAINT chk_cuts_reserved_non_negative CHECK (cuts_reserved_in_period >= 0);

-- Garante no máximo uma assinatura pending_payment por cliente/barbearia
CREATE UNIQUE INDEX IF NOT EXISTS uq_subscriptions_one_pending_per_client_shop
  ON subscriptions(barbershop_id, client_id)
  WHERE status = 'pending_payment';

-- payments: subscription como terceiro alvo válido
ALTER TABLE payments
  ADD COLUMN IF NOT EXISTS subscription_id BIGINT
  REFERENCES subscriptions(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_payments_subscription
  ON payments(barbershop_id, subscription_id)
  WHERE subscription_id IS NOT NULL;

ALTER TABLE payments DROP CONSTRAINT IF EXISTS payment_exactly_one_target;
ALTER TABLE payments ADD CONSTRAINT payment_exactly_one_target CHECK (
  (appointment_id IS NOT NULL AND order_id IS NULL          AND subscription_id IS NULL)
  OR (appointment_id IS NULL  AND order_id IS NOT NULL      AND subscription_id IS NULL)
  OR (appointment_id IS NULL  AND order_id IS NULL          AND subscription_id IS NOT NULL)
);

-- appointments: snapshot de cobertura (decidido no booking, visível no painel)
ALTER TABLE appointments
  ADD COLUMN IF NOT EXISTS subscription_id BIGINT
  REFERENCES subscriptions(id) ON DELETE SET NULL;

ALTER TABLE appointments
  ADD COLUMN IF NOT EXISTS coverage_status coverage_status NOT NULL DEFAULT 'none';

ALTER TABLE appointments
  ADD COLUMN IF NOT EXISTS reserved_subscription_cut BOOLEAN NOT NULL DEFAULT FALSE;

COMMIT;
