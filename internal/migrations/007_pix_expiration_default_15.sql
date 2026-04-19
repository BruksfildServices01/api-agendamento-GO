-- Migration 007: padroniza pix_expiration_minutes = 15 em toda a base
--
-- Contexto: o sistema foi lançado com DEFAULT 3, criando barbearias com
-- pix_expiration_minutes = 3. A decisão de produto é padronizar em 15 minutos.
--
-- Rode no Neon SQL Editor antes do deploy do código.

-- Passo 1: corrige todas as linhas existentes
UPDATE barbershop_payment_configs
SET pix_expiration_minutes = 15
WHERE pix_expiration_minutes != 15;

-- Passo 2: altera o DEFAULT da coluna (sem rewrite de tabela)
ALTER TABLE barbershop_payment_configs
ALTER COLUMN pix_expiration_minutes SET DEFAULT 15;
