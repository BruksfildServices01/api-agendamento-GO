-- Configuração do WhatsApp (Evolution API) por barbearia.
-- Uma barbearia conecta seu número via QR code na Evolution API.
CREATE TABLE IF NOT EXISTS barbershop_whatsapp_configs (
  id            BIGSERIAL PRIMARY KEY,
  barbershop_id BIGINT NOT NULL UNIQUE REFERENCES barbershops(id) ON DELETE CASCADE,
  evolution_url VARCHAR(255) NOT NULL,          -- URL da instância Evolution API
  instance_name VARCHAR(100) NOT NULL,          -- nome da instância
  api_key       VARCHAR(255) NOT NULL,          -- chave de autenticação da instância
  enabled       BOOLEAN NOT NULL DEFAULT true,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_whatsapp_configs_barbershop ON barbershop_whatsapp_configs(barbershop_id);
