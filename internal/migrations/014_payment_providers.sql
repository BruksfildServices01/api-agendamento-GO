-- =============================================================
-- Migration 014 — Multi-provider de pagamento
-- =============================================================
-- Objetivo:
--   1. Criar barbershop_payment_providers: tabela para credenciais
--      criptografadas de providers de pagamento por barbearia.
--   2. Regularizar mp_payment_id em payments: o campo existe no banco
--      de produção (via model GORM) mas nunca foi incluído em nenhuma
--      migration SQL anterior.
--   3. Adicionar provider e provider_payment_id em payments para
--      rastreabilidade genérica de pagamentos multi-provider.
--
-- Esta migration é APENAS ADITIVA:
--   - Nenhuma coluna é removida ou renomeada.
--   - Nenhum dado é movido ou alterado.
--   - O fluxo atual do Mercado Pago não é afetado.
--
-- Como aplicar no Neon:
--   SQL Editor → cole o bloco BEGIN…COMMIT abaixo → Run.
-- =============================================================

BEGIN;

-- ============================================================
-- BARBERSHOP PAYMENT PROVIDERS
-- ============================================================
-- Guarda as credenciais de cada provider de pagamento por barbearia.
-- credentials_encrypted contém o JSON das credenciais criptografado
-- com AES-GCM na aplicação e armazenado em base64 — nunca em claro.
-- webhook_secret é mantido separado para ser lido em hot path
-- (validação HMAC de webhook) sem descriptografar o bloco de credenciais.
-- UNIQUE(barbershop_id, provider) permite múltiplos providers por
-- barbearia sem forçar apenas um ativo — a escolha do provider
-- principal será tratada em fase posterior.

CREATE TABLE IF NOT EXISTS barbershop_payment_providers (
  id                    BIGSERIAL     PRIMARY KEY,
  barbershop_id         BIGINT        NOT NULL REFERENCES barbershops(id) ON DELETE CASCADE,
  provider              VARCHAR(50)   NOT NULL,
  enabled               BOOLEAN       NOT NULL DEFAULT false,
  environment           VARCHAR(20)   NOT NULL DEFAULT 'production',
  credentials_encrypted TEXT,
  webhook_secret        VARCHAR(500),
  created_at            TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
  updated_at            TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
  UNIQUE(barbershop_id, provider)
);

CREATE INDEX IF NOT EXISTS idx_payment_providers_barbershop
  ON barbershop_payment_providers(barbershop_id);

CREATE TRIGGER trg_payment_providers_updated
BEFORE UPDATE ON barbershop_payment_providers
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- PAYMENTS — colunas adicionais
-- ============================================================
-- mp_payment_id: regulariza campo que já existe no banco de produção
--   (presente no model Go) mas nunca foi declarado em migration SQL.
--   Nullable, sem DEFAULT — não altera linhas existentes.
--
-- provider: identifica qual gateway gerou o pagamento.
--   Nullable para compatibilidade com pagamentos históricos.
--   Valores: 'mercadopago' | 'pagbank' | 'stone'
--
-- provider_payment_id: ID externo do provider em formato string,
--   suporta int64 (Mercado Pago) e UUID (PagBank, Stone).
--   Nullable para compatibilidade com pagamentos históricos.

ALTER TABLE payments
  ADD COLUMN IF NOT EXISTS mp_payment_id       BIGINT,
  ADD COLUMN IF NOT EXISTS provider            VARCHAR(50),
  ADD COLUMN IF NOT EXISTS provider_payment_id VARCHAR(100);

CREATE INDEX IF NOT EXISTS idx_payments_mp_payment_id
  ON payments(mp_payment_id)
  WHERE mp_payment_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_payments_provider
  ON payments(barbershop_id, provider)
  WHERE provider IS NOT NULL;

COMMIT;
