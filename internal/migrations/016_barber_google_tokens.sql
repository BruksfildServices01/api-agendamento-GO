-- =============================================================
-- Migration 016 — Google Calendar OAuth tokens por barbeiro
-- =============================================================
-- Cada barbeiro conecta sua própria conta Google individualmente.
-- access_token e refresh_token são armazenados criptografados
-- com AES-256 (mesmo cipher dos payment providers).
-- =============================================================

BEGIN;

CREATE TABLE IF NOT EXISTS barber_google_tokens (
  id            BIGSERIAL    PRIMARY KEY,
  user_id       BIGINT       NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
  barbershop_id BIGINT       NOT NULL REFERENCES barbershops(id) ON DELETE CASCADE,
  access_token  TEXT         NOT NULL,
  refresh_token TEXT         NOT NULL,
  token_expiry  TIMESTAMPTZ  NOT NULL,
  created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_barber_google_tokens_barbershop
  ON barber_google_tokens(barbershop_id);

CREATE TRIGGER trg_barber_google_tokens_updated
BEFORE UPDATE ON barber_google_tokens
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

COMMIT;
