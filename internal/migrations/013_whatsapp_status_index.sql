-- Índice parcial para otimizar o LEFT JOIN do ViewTicket:
--   LEFT JOIN barbershop_whatsapp_instances wi
--     ON wi.barbershop_id = b.id
--     AND wi.barber_id IS NULL
--     AND wi.status = 'connected'
-- A condição wi.status = 'connected' não estava coberta, forçando filter após index scan.
CREATE INDEX IF NOT EXISTS idx_wa_instances_barbershop_connected
  ON barbershop_whatsapp_instances(barbershop_id)
  WHERE barber_id IS NULL AND status = 'connected';
