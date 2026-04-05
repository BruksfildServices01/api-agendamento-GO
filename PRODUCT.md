# Barber Scheduler / Corteon — Documentação Técnica do Produto

## Visão geral

O Barber Scheduler (Corteon) é uma plataforma de gestão e operação comercial para barbearias. O backend cobre o ciclo completo de um atendimento — desde o agendamento público pelo cliente até o fechamento operacional pelo barbeiro — além de venda de produtos, assinaturas, relatórios gerenciais e CRM comportamental.

O sistema foi construído em Go com Gin, GORM e PostgreSQL, seguindo Clean Architecture com separação explícita entre domínio, use cases, handlers e query layers de leitura. Cada domínio tem seu próprio ciclo de vida e não compartilha estado com os outros.

---

## Arquitetura em domínios

| Domínio | Responsabilidade |
|---|---|
| `appointment` | Ciclo de vida do agendamento: criação, conclusão, cancelamento, no-show |
| `order` | Venda de produtos separada do agendamento |
| `payment` | PIX, confirmação de pagamento, expiração e relatórios |
| `subscription` | Assinaturas, planos e consumo de cortes |
| `ticket` | Link público do agendamento para o cliente |
| `cart` | Carrinho de produtos persistido em Postgres |
| `metrics` | Métricas comportamentais de clientes (base do CRM) |
| `audit` | Rastreabilidade de ações sensíveis por tenant |
| `query/crm` | Leitura consolidada do perfil do cliente |
| `query/daypanel` | Leitura operacional do dia para o barbeiro |
| `query/dashboard` | Indicadores macro por período |
| `query/financial` | Receita realizada, esperada e perdas |
| `query/impact` | ROI, retenção e crescimento |

---

## 1. Autenticação e contexto do tenant

### Por que existe

O sistema é multi-tenant por design: cada barbearia é um tenant isolado. Toda rota autenticada carrega `barbershop_id` via JWT, e o middleware garante que nenhuma operação de uma barbearia possa acessar dados de outra.

### Como funciona o registro

O `POST /api/auth/register` executa uma transação única que cria, atomicamente:

1. A barbearia com timezone padrão `America/Sao_Paulo`
2. O usuário owner com senha em bcrypt
3. A configuração de cobrança padrão (payment config)
4. Horários de trabalho padrão (segunda a sexta, 09:00–17:00)
5. Um serviço inicial ("Corte de cabelo", 30 min, R$ 50,00)

Se qualquer etapa falhar, nada é persistido. O slug é validado antes da transação para evitar colisões.

### Endpoints

```
POST /api/auth/register
```
Registra nova barbearia. Cria o tenant completo em uma transação. Retorna usuário, barbearia e JWT.

**Body:**
```json
{
  "barbershop_name": "Barbearia do João",
  "barbershop_slug": "barbearia-joao",
  "barbershop_phone": "11999990000",
  "barbershop_address": "Rua Exemplo, 123",
  "name": "João Silva",
  "email": "joao@email.com",
  "password": "minimo6chars",
  "phone": "11999990001"
}
```

```
POST /api/auth/login
```
Autentica email e senha com bcrypt. Retorna JWT com validade de 24 horas contendo `sub` (user ID), `barbershopId` e `role`.

### JWT e middleware

O token carrega `barbershopId` e `role`. O middleware de autenticação valida a assinatura, extrai os claims e injeta `barbershop_id` e `user_id` no contexto do Gin. Todas as rotas autenticadas leem esses valores do contexto — nunca do body da requisição.

---

## 2. Configuração da barbearia

### Por que existe

A barbearia tem parâmetros operacionais que afetam outros fluxos: `timezone` é a fonte de verdade para todos os cálculos de horário; `min_advance_minutes` define a antecedência mínima para agendamentos públicos.

### Endpoints

```
GET /api/me/barbershop
```
Retorna os dados atuais da barbearia do tenant autenticado.

```
PUT /api/me/barbershop
```
Atualiza dados da barbearia. Alterações no `timezone` afetam imediatamente a leitura do painel do dia e os cálculos de disponibilidade.

---

## 3. Catálogo — Serviços, Produtos e Sugestão Comercial

### Por que existe

Serviços e produtos são entidades distintas com propósitos distintos:

- **Serviço** é o que o barbeiro executa no cliente (corte, barba, hidratação). Tem duração e preço. É a unidade do agendamento.
- **Produto** é o que pode ser vendido fisicamente (pomada, shampoo). Tem estoque e preço. É a unidade do pedido/carrinho.
- **Sugestão** é o vínculo entre um serviço e um produto: quando o cliente agenda um corte, o sistema pode sugerir a pomada usada no atendimento. É uma feature de venda adicional automatizada.

### Endpoints — Serviços

```
GET  /api/me/services
POST /api/me/services
PUT  /api/me/services/:id
```
CRUD administrativo de serviços. O `POST` cria o serviço com duração, preço e categoria. O `PUT` permite ativar/desativar além de atualizar metadados.

```
GET /api/public/:slug/services
```
Leitura pública do catálogo de serviços, filtrado por serviços ativos. Usado pela jornada pública do cliente para montar a tela de agendamento.

### Endpoints — Produtos

```
GET  /api/me/products
POST /api/me/products
PUT  /api/me/products/:id
```
CRUD administrativo de produtos. Controla ativação, visibilidade e estoque.

```
GET /api/public/:slug/products
```
Leitura pública dos produtos visíveis e com estoque disponível. Alimenta o catálogo do carrinho público.

### Endpoints — Sugestão Comercial

```
GET    /api/me/services/:id/suggestion
PUT    /api/me/services/:id/suggestion
DELETE /api/me/services/:id/suggestion
```
Gestão interna do vínculo entre serviço e produto sugerido. O barbeiro define qual produto recomendar para cada serviço.

```
GET /api/public/:slug/services/:id/suggestion
```
Leitura pública da sugestão. Retorna o produto sugerido apenas se ele estiver ativo, visível e com estoque. É chamado pelo frontend após o cliente escolher um serviço para exibir a recomendação.

---

## 4. Horários de trabalho e disponibilidade

### Por que existe

A disponibilidade de slots de agendamento depende dos horários configurados por barbeiro e dia da semana. A lógica leva em conta horário de almoço e valida conflitos com agendamentos já existentes. O `timezone` da barbearia é usado como fonte de verdade — todos os horários são interpretados no fuso local da barbearia, independente do fuso do cliente que está acessando.

### Endpoints

```
GET /api/me/working-hours
PUT /api/me/working-hours
```
Leitura e atualização dos horários de trabalho. Cada dia da semana pode ser ativado/desativado individualmente, com horário de início, fim e intervalo de almoço opcional.

```
GET /api/public/:slug/availability?date=YYYY-MM-DD&service_id=1
```
Retorna os slots disponíveis para uma data e serviço específicos. Calcula a grade de horários com base nos horários de trabalho, duração do serviço e agendamentos já existentes. Responde com `date`, `timezone` e `slots`.

---

## 5. Agendamento — quatro formas de criar

### Por que existe

O sistema precisa cobrir cenários distintos de criação de agendamento, cada um com suas regras:

| Forma | Endpoint | Quem usa | Diferencial |
|---|---|---|---|
| Público padrão | `POST /api/public/:slug/appointments` | Cliente externo | Resolve barbeiro pelo slug |
| Checkout orquestrado | `POST /api/public/:slug/checkout` | Cliente externo | Agenda + pedido + ticket em uma chamada |
| Privado autenticado | `POST /api/me/appointments` | Barbeiro | Aplica políticas de cobrança e CRM |
| Interno | `POST /api/me/internal-appointments` | Barbeiro | `start_time`/`end_time` explícitos, sem política |

### Agendamento público padrão

```
POST /api/public/:slug/appointments
```
Criado pelo cliente, sem autenticação. O barbeiro é resolvido internamente pelo slug (sempre o owner da barbearia). Aplica as mesmas regras de validação do agendamento privado: antecedência mínima, horário de trabalho, conflito de horário. Suporta `X-Idempotency-Key` para evitar duplicatas em retentativas.

**Body:**
```json
{
  "service_id": 1,
  "date": "2026-04-10",
  "time": "10:00",
  "client_name": "João Silva",
  "client_phone": "11999990001",
  "client_email": "joao@email.com",
  "notes": "Preferência por tesoura"
}
```

### Checkout orquestrado

```
POST /api/public/:slug/checkout
```
O endpoint mais completo da jornada pública. Em uma única chamada, ele:

1. Cria o agendamento
2. Faz checkout do carrinho (se houver `cart_key` com itens)
3. Gera o ticket público do agendamento
4. Dispara notificação por email (se configurado)
5. Retorna URLs prontas para pagamento PIX e para o ticket

Responde com `next_step` indicando se há pagamento pendente de appointment, de order, ambos, ou se o checkout foi concluído sem cobrança.

**Body:**
```json
{
  "service_id": 1,
  "date": "2026-04-10",
  "time": "10:00",
  "client_name": "João",
  "client_phone": "11999990001",
  "client_email": "joao@email.com",
  "cart_key": "abc123"
}
```

### Agendamento privado autenticado

```
POST /api/me/appointments
```
Criado pelo barbeiro na área autenticada. Além das validações de horário, aplica:

- **Política de cobrança**: verifica se o cliente (pela categoria CRM) exige pagamento antecipado
- **Assinatura ativa**: se o cliente tem assinatura que cobre o serviço, o agendamento é criado como gratuito (não exige PIX)
- **Idempotência**: suporta `X-Idempotency-Key`

### Agendamento interno

```
POST /api/me/internal-appointments
```
Criado pelo barbeiro para casos onde a lógica padrão não se aplica: encaixes manuais, bloqueios de agenda, registros retroativos. Recebe `start_time` e `end_time` como timestamps explícitos (sem calcular duração pelo serviço), não aplica política de cobrança e aceita `payment_intent` direto (`paid` ou `pay_later`).

### Leitura de agendamentos

```
GET /api/me/appointments/date?date=YYYY-MM-DD
GET /api/me/appointments/month?year=2026&month=4
```
Listagem por dia e por mês para o painel do barbeiro.

### Ações sobre agendamentos

```
PUT /api/me/appointments/:id/cancel
```
Cancela o agendamento. Atualiza métricas do cliente (cancellation_rate). Se o cancelamento ocorrer dentro de 24h do horário agendado, registra `late_cancellation` nas métricas.

```
PUT /api/me/appointments/:id/no-show
```
Marca o cliente como ausente. Atualiza métricas com `no_show`. Impacta a categoria CRM do cliente diretamente.

```
PUT /api/me/appointments/:id/complete
```
Encerra operacionalmente o atendimento. Este é o endpoint mais rico do fluxo interno. Permite:

- Registrar o serviço efetivamente realizado (pode diferir do agendado)
- Informar o valor final cobrado
- Registrar a forma de pagamento
- Registrar venda adicional de produtos (cria order vinculada)
- Consumir a assinatura do cliente quando aplicável
- Confirmar cobrança normal quando a assinatura não cobre o serviço

O backend valida que, se o agendamento exigia pagamento PIX antecipado, o pagamento esteja confirmado antes de permitir a conclusão. Responde com o appointment atualizado, o fechamento operacional e o resultado do consumo de assinatura.

---

## 6. Ticket público do agendamento

### Por que existe

Após criar um agendamento, o cliente recebe um link com um token de 64 caracteres (32 bytes aleatórios em hex) que expira no horário do próprio agendamento. Esse token é a interface pública do cliente para gerenciar seu compromisso sem precisar de conta ou autenticação.

### Endpoints

```
GET /api/public/ticket/:token
```
Retorna os dados do agendamento vinculado ao token. Retorna `410 Gone` se o token estiver expirado (horário do agendamento já passou).

```
DELETE /api/public/ticket/:token
```
Cancela o agendamento via ticket. Valida janela de cancelamento (não permite cancelar muito próximo do horário). Atualiza métricas. Retorna `204 No Content`.

```
PATCH /api/public/ticket/:token
```
Reagenda o agendamento para uma nova data/hora. Valida conflito, horário de trabalho e antecedência mínima. **O token rotaciona**: após o reagendamento bem-sucedido, o token antigo é invalidado e um novo token é retornado na resposta. O cliente deve salvar o novo token.

**Body:**
```json
{
  "date": "2026-04-15",
  "time": "14:00"
}
```

---

## 7. Ajuste pós-fechamento

### Por que existe

Erros acontecem. O barbeiro pode registrar o valor errado ou a forma de pagamento incorreta no fechamento. O sistema permite corrigir esses dados dentro de uma janela de tempo, sem sobrescrever o registro original — cada ajuste é gravado como um evento separado com motivo obrigatório.

### Endpoint

```
POST /api/me/appointments/:id/closure/adjustment
```
Cria um ajuste sobre o fechamento de um atendimento já concluído. Campos opcionais (enviar apenas o que precisa ser corrigido):

- `delta_final_amount_cents` — novo valor final
- `delta_payment_method` — nova forma de pagamento
- `delta_operational_note` — nova nota operacional
- `reason` (**obrigatório**) — motivo do ajuste

Erros possíveis: `closure_not_found`, `adjustment_window_expired`, `no_adjustment_fields`.

---

## 8. Carrinho e jornada comercial pública

### Por que existe

O sistema não é apenas uma agenda: permite que o cliente adicione produtos ao carrinho durante a jornada pública e faça o checkout junto com o agendamento. O carrinho é persistido em Postgres (não em memória), o que garante consistência em ambiente multi-instância.

O carrinho é identificado por `X-Cart-Key` (header obrigatório) e está sempre vinculado a uma barbearia específica via slug.

### Endpoints

```
GET    /api/public/:slug/cart
POST   /api/public/:slug/cart/items
DELETE /api/public/:slug/cart/items/:productId
POST   /api/public/:slug/cart/checkout
```
CRUD do carrinho e checkout independente. O checkout do carrinho cria uma `order` sem agendamento associado. Para checkout combinado (agendamento + carrinho), usar `POST /api/public/:slug/checkout`.

---

## 9. Pedidos

### Por que existe

`order` é o domínio de venda de produtos, separado de `appointment`. Um pedido pode existir sem agendamento (venda balcão), pode ser criado pelo barbeiro na área autenticada, ou pode nascer do checkout do carrinho público.

### Endpoints

```
POST /api/me/orders
GET  /api/me/orders
GET  /api/me/orders/:id
```
CRUD administrativo de pedidos. O `POST` cria um pedido com lista de itens (product_id + quantity). O sistema verifica estoque e calcula o total.

```
POST /api/me/orders/:id/payment/pix
POST /api/public/:slug/orders/:id/payment/pix
```
Gera cobrança PIX para um pedido. Versão pública é protegida por rate limit (30 req/min por IP + slug). Ambas criam o registro de pagamento e retornam o QR Code.

---

## 10. Pagamentos e PIX

### Por que existe

O sistema centraliza toda a lógica de pagamento em um domínio próprio. PIX pode ser gerado para agendamentos ou pedidos. A confirmação é recebida via webhook e processada de forma idempotente.

### Gateway selecionável

O gateway PIX é configurado por variável de ambiente (`PIX_PROVIDER`):

- `mock` (padrão) — retorna dados falsos para desenvolvimento
- `efi` — integração real com a API Efí (ex-Gerencianet) via OAuth2

### Endpoints

```
POST /api/public/:slug/appointments/:id/payment/pix
```
Gera PIX para um agendamento público. Protegido por rate limit. Retorna `txid`, QR Code e expiração.

```
POST /api/webhooks/pix
```
Recebe confirmação de pagamento da provedora PIX. Autenticado por HMAC (`PIX_WEBHOOK_SECRET`). O handler é tolerante: responde sempre `200 OK` independente do resultado interno, para evitar reentradas. Processa apenas eventos com `event: "paid"` e `txid` não vazio.

```
GET /api/me/payments
```
Lista todos os pagamentos do tenant com filtros.

```
GET /api/me/payments/summary?from=YYYY-MM-DD&to=YYYY-MM-DD
```
Resumo financeiro de pagamentos no período. Datas são interpretadas no timezone da barbearia. Aceita também RFC3339 para precisão de instante.

```
GET /api/me/summary
```
Resumo operacional rápido: total de agendamentos, confirmados, cancelados e no-show.

---

## 11. Assinaturas e planos

### Por que existe

O sistema suporta um modelo de assinatura onde clientes pagam um plano mensal e têm direito a um número fixo de cortes incluídos. Isso impacta diretamente o fluxo de cobrança: se o cliente tem assinatura ativa cobrindo o serviço, o agendamento é criado sem exigir pagamento PIX.

A regra de assinatura ativa usa condição canônica com verificação de período (`status = 'active' AND current_period_start <= NOW() AND current_period_end > NOW()`), aplicada uniformemente em todas as queries do sistema.

### Endpoints

```
POST /api/me/plans
GET  /api/me/plans
```
Criação e listagem de planos. Cada plano define nome, preço, número de cortes incluídos e quais serviços cobre.

```
POST   /api/me/subscriptions
GET    /api/me/subscriptions/:clientID
DELETE /api/me/subscriptions/:clientID
```
Ativação, consulta e cancelamento de assinatura de um cliente. Ao ativar, define `current_period_start`, `current_period_end` e zera `cuts_used_in_period`. O consumo ocorre apenas na conclusão do atendimento, não na criação.

---

## 12. Políticas de cobrança

### Por que existe

Nem todo cliente paga antecipado. A política de cobrança define, por categoria CRM, se o pagamento PIX é obrigatório antes do atendimento, opcional ou automático. O barbeiro configura essa política por tenant.

### Endpoints

```
GET /api/me/payment-policies
PUT /api/me/payment-policies
```
Leitura e atualização das políticas. Define o comportamento por categoria (`new`, `regular`, `trusted`, `at_risk`) e o padrão geral.

---

## 13. Clientes e CRM

### Por que existe

O sistema mantém métricas comportamentais por cliente: total de agendamentos, taxa de comparecimento, cancelamentos tardios, no-shows, receita gerada. Essas métricas são a base para categorização automática do cliente, que por sua vez determina a política de cobrança aplicada.

### Categorias

| Categoria | Significado |
|---|---|
| `new` | Nunca veio ou histórico insuficiente |
| `regular` | Frequência normal, sem flags negativas |
| `trusted` | Alta taxa de comparecimento (≥90% com mínimo 5 visitas) e premium |
| `at_risk` | Histórico de no-show ou cancelamento tardio recorrente |

### Endpoints

```
GET /api/me/clients
```
Lista todos os clientes do tenant com categoria atual e flag de premium (assinatura ativa).

```
GET /api/me/clients/:id/history
```
Histórico consolidado do cliente: todos os atendimentos, status, valores, serviços.

```
GET /api/me/clients/:id/category
```
Categoria calculada automaticamente com base nas métricas comportamentais.

```
PUT /api/me/clients/:id/category
```
Override manual da categoria. O barbeiro pode forçar uma categoria diferente da calculada, com expiração opcional em dias. Após a expiração, o sistema volta a calcular automaticamente. Retorna `204 No Content`.

**Body:**
```json
{
  "category": "trusted",
  "expires_in_days": 30
}
```

```
GET /api/me/clients/:id/crm
```
Visão completa do cliente para o CRM: identidade, métricas consolidadas, flags comportamentais (`reliable`, `premium`, `attention`), assinatura ativa e política operacional derivada. Este endpoint é a leitura mais rica do sistema sobre um cliente específico.

---

## 14. Painel do Dia

### Por que existe

O barbeiro precisa de uma visão operacional do dia atual: quem vem, a que horas, o que vai fazer, se há pagamento pendente, se o cliente tem assinatura, se há sugestão de produto e se há pedido pré-pago. O Painel do Dia reúne tudo isso em cards prontos para consumo pelo frontend, sem precisar cruzar múltiplas chamadas.

### Endpoint

```
GET /api/me/day-panel?date=YYYY-MM-DD&barber_id=1
```
`date` padrão é hoje no timezone da barbearia. `barber_id` é opcional (retorna todos os barbeiros se omitido).

Cada card retorna: dados do cliente, serviço, horário, status, pagamento, sugestão comercial, pedido antecipado, assinatura e flags operacionais.

---

## 15. Dashboard, Financeiro e Impacto

Três visões gerenciais com query layers dedicadas e independentes.

### Dashboard

```
GET /api/me/dashboard?period=day|week|month
```
Indicadores macro do período: total de atendimentos, receita gerada, clientes novos vs recorrentes, ranking de serviços e produtos mais vendidos.

### Financeiro

```
GET /api/me/financial?period=week|month
```
Breakdown financeiro detalhado: receita realizada (pagamentos confirmados), expectativa (agendamentos futuros confirmados), presumido (agendamentos sem cobrança) e perdas (cancelamentos e no-shows com valor estimado).

### Impacto / ROI

```
GET /api/me/impact?period=week|month
```
Indicadores de crescimento e retenção: taxa de retenção de clientes, crescimento de receita, ganhos indiretos via assinatura, impacto de no-shows e cancelamentos.

---

## 16. Auditoria

### Por que existe

Ações sensíveis são registradas de forma assíncrona via dispatcher com buffer de canal. O registro não bloqueia o fluxo principal — se o buffer estiver cheio, o evento é descartado silenciosamente (fail-open). Cada log contém barbearia, usuário, ação, entidade, ID da entidade e metadata JSON opcional.

Ações auditadas: `appointment_created`, `appointment_cancelled`, `appointment_no_show`, `payment_created`, `payment_confirmed`, `payment_expired`, `subscription_activated`, `subscription_cancelled`, `working_hours_updated`, `payment_policy_updated`, `closure_adjusted`, e outras.

### Endpoint

```
GET /api/me/audit-logs?action=...&entity=...&from=YYYY-MM-DD&to=YYYY-MM-DD&page=1&limit=50
```
Listagem paginada. Todos os filtros são opcionais. Sempre filtrado pelo tenant autenticado — impossível acessar logs de outra barbearia.

---

## 17. Jobs automáticos

### Por que existe

Dois processos precisam rodar de forma periódica sem intervenção humana e sem duplicar execução em ambientes com múltiplas instâncias.

### Leader lock em Postgres

Antes de executar, cada job tenta adquirir um advisory lock no Postgres. Se outra instância já tiver o lock, o job é pulado naquele ciclo. O lock tem TTL de 2 minutos e é renovado a cada execução bem-sucedida.

### Jobs

**Expiração de pagamentos** — Roda a cada minuto. Busca pagamentos PIX com status `pending` e `expires_at < NOW()`. Para cada um, cancela o pagamento e atualiza o agendamento vinculado.

**Marcação de no-show** — Roda a cada minuto. Busca agendamentos com status `scheduled` ou `awaiting_payment` cujo `start_time` já passou. Marca como `no_show` e atualiza as métricas do cliente.

---

## 18. Mecanismos transversais

### Idempotência

Rotas críticas (`POST /appointments`, `POST /checkout`, `POST /payment/pix`) aceitam `X-Idempotency-Key` no header. Se a mesma chave for usada duas vezes, a segunda requisição retorna `409 Conflict` com `error_code: duplicate_request`. Chaves são armazenadas no Postgres com TTL.

### Rate limiting

Rotas públicas sensíveis (geração de PIX) têm rate limit de 30 req/min por IP + slug. Se `REDIS_URL` estiver configurado, o contador é compartilhado entre todas as instâncias (distribuído). Caso contrário, usa contador in-memory por instância. Em caso de falha do Redis, o sistema permite a requisição (fail-open).

### Timezone

O timezone da barbearia é a fonte de verdade para todos os cálculos de horário. Datas enviadas pelo cliente são interpretadas no fuso da barbearia, não no fuso do servidor. Slots de disponibilidade, horários de trabalho e validações de antecedência mínima usam sempre `time.ParseInLocation` com o timezone do tenant.

### Email e notificações

Se `EMAIL_ENABLED=true` e as variáveis SMTP estiverem configuradas, o sistema envia:
- Confirmação de agendamento (com ICS) via checkout orquestrado
- Notificação de cancelamento via ticket
- Notificação de reagendamento via ticket

Se o email não estiver configurado, as chamadas caem em um `NoopNotifier` que descarta silenciosamente, sem retornar erro.

---

## Variáveis de ambiente

| Variável | Obrigatória | Descrição |
|---|---|---|
| `DATABASE_URL` | Sim | String de conexão PostgreSQL |
| `JWT_SECRET` | Sim | Segredo para assinar tokens JWT |
| `PIX_WEBHOOK_SECRET` | Sim | Segredo HMAC para autenticar webhooks PIX |
| `SERVER_PORT` | Não | Porta do servidor (padrão: 8080) |
| `CORS_ALLOWED_ORIGINS` | Não | Origens permitidas em CSV |
| `EMAIL_ENABLED` | Não | `true` para ativar envio de email |
| `EMAIL_FROM` | Se email | Endereço de origem |
| `SMTP_HOST` | Se email | Host SMTP (ex: smtp-relay.brevo.com) |
| `SMTP_PORT` | Se email | Porta SMTP |
| `SMTP_USER` | Se email | Usuário SMTP |
| `SMTP_PASS` | Se email | Senha SMTP |
| `PIX_PROVIDER` | Não | `mock` (padrão) ou `efi` |
| `EFI_CLIENT_ID` | Se efi | Client ID da API Efí |
| `EFI_CLIENT_SECRET` | Se efi | Client Secret da API Efí |
| `EFI_PIX_KEY` | Se efi | Chave PIX cadastrada na Efí |
| `REDIS_URL` | Não | URL Redis para rate limit distribuído |

---

## Referência completa de endpoints

### Públicos (sem autenticação)

| Método | Rota | Descrição |
|---|---|---|
| POST | `/api/auth/register` | Registra barbearia e owner |
| POST | `/api/auth/login` | Autentica e retorna JWT |
| GET | `/api/public/:slug/services` | Lista serviços ativos |
| GET | `/api/public/:slug/products` | Lista produtos disponíveis |
| GET | `/api/public/:slug/services/:id/suggestion` | Sugestão de produto por serviço |
| GET | `/api/public/:slug/availability` | Slots disponíveis por data e serviço |
| POST | `/api/public/:slug/appointments` | Agendamento público simples |
| POST | `/api/public/:slug/appointments/:id/payment/pix` | Gera PIX para agendamento |
| GET | `/api/public/:slug/cart` | Lê carrinho |
| POST | `/api/public/:slug/cart/items` | Adiciona produto ao carrinho |
| DELETE | `/api/public/:slug/cart/items/:productId` | Remove produto do carrinho |
| POST | `/api/public/:slug/cart/checkout` | Checkout do carrinho (só produtos) |
| POST | `/api/public/:slug/checkout` | Checkout orquestrado (agenda + carrinho + ticket) |
| POST | `/api/public/:slug/orders/:id/payment/pix` | Gera PIX para pedido público |
| GET | `/api/public/ticket/:token` | Visualiza ticket do agendamento |
| DELETE | `/api/public/ticket/:token` | Cancela via ticket |
| PATCH | `/api/public/ticket/:token` | Reagenda via ticket (token rotaciona) |
| POST | `/api/webhooks/pix` | Webhook de confirmação PIX |

### Autenticados — `/api/me`

| Método | Rota | Descrição |
|---|---|---|
| GET | `/api/me` | Dados do usuário autenticado |
| GET | `/api/me/barbershop` | Dados da barbearia |
| PUT | `/api/me/barbershop` | Atualiza dados da barbearia |
| GET | `/api/me/services` | Lista serviços |
| POST | `/api/me/services` | Cria serviço |
| PUT | `/api/me/services/:id` | Atualiza serviço |
| GET | `/api/me/services/:id/suggestion` | Lê sugestão do serviço |
| PUT | `/api/me/services/:id/suggestion` | Define sugestão do serviço |
| DELETE | `/api/me/services/:id/suggestion` | Remove sugestão do serviço |
| GET | `/api/me/products` | Lista produtos |
| POST | `/api/me/products` | Cria produto |
| PUT | `/api/me/products/:id` | Atualiza produto |
| GET | `/api/me/working-hours` | Lê horários de trabalho |
| PUT | `/api/me/working-hours` | Atualiza horários de trabalho |
| GET | `/api/me/payment-policies` | Lê políticas de cobrança |
| PUT | `/api/me/payment-policies` | Atualiza políticas de cobrança |
| POST | `/api/me/appointments` | Agendamento privado autenticado |
| PUT | `/api/me/appointments/:id/complete` | Conclui atendimento operacionalmente |
| PUT | `/api/me/appointments/:id/cancel` | Cancela agendamento |
| PUT | `/api/me/appointments/:id/no-show` | Marca cliente como ausente |
| POST | `/api/me/appointments/:id/closure/adjustment` | Ajuste pós-fechamento |
| GET | `/api/me/appointments/date` | Lista agendamentos por dia |
| GET | `/api/me/appointments/month` | Lista agendamentos por mês |
| POST | `/api/me/internal-appointments` | Agendamento interno (encaixe/bloqueio) |
| GET | `/api/me/payments` | Lista pagamentos |
| GET | `/api/me/payments/summary` | Resumo financeiro de pagamentos |
| GET | `/api/me/summary` | Resumo operacional rápido |
| POST | `/api/me/orders` | Cria pedido |
| GET | `/api/me/orders` | Lista pedidos |
| GET | `/api/me/orders/:id` | Busca pedido por ID |
| POST | `/api/me/orders/:id/payment/pix` | Gera PIX para pedido |
| GET | `/api/me/clients` | Lista clientes com categoria |
| GET | `/api/me/clients/:id/history` | Histórico do cliente |
| GET | `/api/me/clients/:id/category` | Categoria CRM do cliente |
| PUT | `/api/me/clients/:id/category` | Override manual de categoria |
| GET | `/api/me/clients/:id/crm` | Perfil CRM completo do cliente |
| POST | `/api/me/plans` | Cria plano de assinatura |
| GET | `/api/me/plans` | Lista planos |
| POST | `/api/me/subscriptions` | Ativa assinatura de cliente |
| GET | `/api/me/subscriptions/:clientID` | Lê assinatura ativa do cliente |
| DELETE | `/api/me/subscriptions/:clientID` | Cancela assinatura |
| GET | `/api/me/audit-logs` | Lista logs de auditoria |
| GET | `/api/me/day-panel` | Painel operacional do dia |
| GET | `/api/me/dashboard` | Dashboard por período |
| GET | `/api/me/financial` | Relatório financeiro por período |
| GET | `/api/me/impact` | Relatório de impacto/ROI por período |
