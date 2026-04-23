-- Migration 008: persiste tours vistos por usuário no banco
--
-- Contexto: tour state vivia só em localStorage, que é apagado por browsers
-- mobile (Safari iOS) após inatividade. Sem persistência no servidor, o tutorial
-- reabre para usuários antigos ao voltarem ao sistema.
--
-- Rode no Neon SQL Editor antes do deploy do código.

ALTER TABLE users ADD COLUMN IF NOT EXISTS seen_tours TEXT NOT NULL DEFAULT '[]';

-- Marca todos os usuários existentes como tendo visto todos os tours,
-- evitando que o tutorial reabra para quem já usou o sistema.
UPDATE users SET seen_tours = '["agenda","assinaturas","catalogo-produtos","catalogo-servicos","catalogo-sugestoes","clientes","dashboard","painel-do-dia","financeiro","impacto","pedidos","politicas-pagamento","extrato","minha-conta","horarios-trabalho"]';
