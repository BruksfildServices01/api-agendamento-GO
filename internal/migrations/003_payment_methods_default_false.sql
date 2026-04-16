-- Corrige o default das colunas de formas de pagamento de true para false.
-- Novas barbearias devem começar sem nenhum método de pagamento online ativo.
ALTER TABLE barbershop_payment_configs
  ALTER COLUMN accept_cash    SET DEFAULT false,
  ALTER COLUMN accept_pix     SET DEFAULT false,
  ALTER COLUMN accept_credit  SET DEFAULT false,
  ALTER COLUMN accept_debit   SET DEFAULT false;
