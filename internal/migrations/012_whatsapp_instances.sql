-- Remove configuração antiga (URL/apikey por barbearia — expunha complexidade ao barbeiro)
DROP TABLE IF EXISTS barbershop_whatsapp_configs;

-- Nova tabela: instâncias WhatsApp por barbearia (ou por barbeiro no futuro)
-- barber_id NULL = instância da barbearia inteira (modelo atual)
-- barber_id preenchido = instância individual (plano multi-barbeiro futuro)
CREATE TABLE IF NOT EXISTS barbershop_whatsapp_instances (
  id            BIGSERIAL PRIMARY KEY,
  barbershop_id BIGINT NOT NULL REFERENCES barbershops(id) ON DELETE CASCADE,
  barber_id     BIGINT REFERENCES users(id) ON DELETE CASCADE,
  instance_name VARCHAR(100) NOT NULL UNIQUE,
  phone         VARCHAR(20),
  status        VARCHAR(20) NOT NULL DEFAULT 'disconnected',
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Garante um registro por (barbearia, barbeiro) — barber_id NULL = barbearia inteira
CREATE UNIQUE INDEX IF NOT EXISTS uq_whatsapp_barbershop_barber
  ON barbershop_whatsapp_instances(barbershop_id, COALESCE(barber_id, 0));
