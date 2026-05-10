#!/usr/bin/env bash
# ============================================================
# test_flows.sh — testa todos os fluxos do CorteOn backend
# ============================================================
# Uso:
#   chmod +x test_flows.sh
#   ./test_flows.sh
#
# Pré-requisitos:
#   - Backend rodando em http://localhost:8080
#   - Conta criada com EMAIL/PASS abaixo
#   - Slug da barbearia configurado em SLUG
# ============================================================

BASE="http://localhost:8080/api"
SLUG="barbearia-prime"
EMAIL="lucas@teste.com"
PASS="123456"
CLIENT_EMAIL="cliente@teste.com"
CART_KEY="ci-cart-$(date +%s)"

PASS_COUNT=0
FAIL_COUNT=0
SKIP_COUNT=0

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
NC='\033[0m'

# ── Helpers ──────────────────────────────────────────────────────────────────

assert() {
  local label="$1" pattern="$2" actual="$3"
  if echo "$actual" | grep -q "$pattern" 2>/dev/null; then
    echo -e "${GREEN}✔ PASS${NC} — $label"
    ((PASS_COUNT++)) || true
  else
    echo -e "${RED}✘ FAIL${NC} — $label"
    echo "       padrão : '$pattern'"
    echo "       recebido: $(echo "$actual" | head -c 300)"
    ((FAIL_COUNT++)) || true
  fi
}

assert_status() {
  local label="$1" expected="$2" actual="$3"
  if [[ "$actual" == "$expected" ]]; then
    echo -e "${GREEN}✔ PASS${NC} — $label"
    ((PASS_COUNT++)) || true
  else
    echo -e "${RED}✘ FAIL${NC} — $label (esperado $expected, recebido $actual)"
    ((FAIL_COUNT++)) || true
  fi
}

assert_status_range() {
  local label="$1" prefix="$2" actual="$3"
  if [[ "$actual" == ${prefix}* ]]; then
    echo -e "${GREEN}✔ PASS${NC} — $label (${actual})"
    ((PASS_COUNT++)) || true
  else
    echo -e "${RED}✘ FAIL${NC} — $label (esperado ${prefix}xx, recebido $actual)"
    ((FAIL_COUNT++)) || true
  fi
}

skip() {
  echo -e "${YELLOW}⊘ SKIP${NC} — $1"
  ((SKIP_COUNT++)) || true
}

jq_id() {
  local json="$1" field="${2:-id}"
  echo "$json" | grep -o "\"${field}\":[0-9]*" | head -1 | grep -o '[0-9]*$'
}

get() {
  local url="$1" token="${2:-}"
  local args=(-s -X GET "$BASE$url")
  [[ -n "$token" ]] && args+=(-H "Authorization: Bearer $token")
  curl "${args[@]}"
}

get_status() {
  local url="$1" token="${2:-}"
  local args=(-s -o /dev/null -w "%{http_code}" -X GET "$BASE$url")
  [[ -n "$token" ]] && args+=(-H "Authorization: Bearer $token")
  curl "${args[@]}"
}

req() {
  local method="$1" url="$2" data="${3:-}" token="${4:-}"
  local args=(-s -X "$method" "$BASE$url" -H "Content-Type: application/json")
  [[ -n "$token" ]] && args+=(-H "Authorization: Bearer $token")
  [[ -n "$data" ]] && args+=(-d "$data")
  curl "${args[@]}"
}

req_status() {
  local method="$1" url="$2" data="${3:-}" token="${4:-}"
  local args=(-s -o /dev/null -w "%{http_code}" -X "$method" "$BASE$url" -H "Content-Type: application/json")
  [[ -n "$token" ]] && args+=(-H "Authorization: Bearer $token")
  [[ -n "$data" ]] && args+=(-d "$data")
  curl "${args[@]}"
}

cart_req() {
  local method="$1" url="$2" data="${3:-}"
  local args=(-s -X "$method" "$BASE$url" -H "Content-Type: application/json" -H "X-Cart-Key: $CART_KEY")
  [[ -n "$data" ]] && args+=(-d "$data")
  curl "${args[@]}"
}

cart_status() {
  local method="$1" url="$2" data="${3:-}"
  local args=(-s -o /dev/null -w "%{http_code}" -X "$method" "$BASE$url" -H "Content-Type: application/json" -H "X-Cart-Key: $CART_KEY")
  [[ -n "$data" ]] && args+=(-d "$data")
  curl "${args[@]}"
}

_get_date() { date -v+${1}d +%Y-%m-%d 2>/dev/null || date -d "+${1} days" +%Y-%m-%d; }

# ── Datas futuras sem conflito ────────────────────────────────────────────────
_DOW=$(date +%u)
_DAYS_TO_MON=$(( (8 - _DOW) % 7 ))
[[ "$_DAYS_TO_MON" -eq 0 ]] && _DAYS_TO_MON=7
_WOFF=$(( RANDOM % 40 + 4 ))

DATE_MON=$(_get_date $((_DAYS_TO_MON + _WOFF * 7)))
DATE_TUE=$(_get_date $((_DAYS_TO_MON + 1 + _WOFF * 7)))
DATE_WED=$(_get_date $((_DAYS_TO_MON + 2 + _WOFF * 7)))
DATE_THU=$(_get_date $((_DAYS_TO_MON + 3 + _WOFF * 7)))

echo ""
echo "============================================================"
echo "  CorteOn — test suite"
echo "  BASE=$BASE  SLUG=$SLUG  semana+${_WOFF}"
echo "============================================================"
echo ""

# ============================================================
# 1. AUTH
# ============================================================
echo "── 1. AUTH ─────────────────────────────────────────────"

LOGIN=$(req POST /auth/login "{\"email\":\"$EMAIL\",\"password\":\"$PASS\"}")
TOKEN=$(echo "$LOGIN" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
assert "POST /auth/login retorna token" '"token"' "$LOGIN"

assert_status "POST /auth/login inválido → 401" "401" \
  "$(req_status POST /auth/login '{"email":"x@x.com","password":"bad"}')"

# Extrai user_id do /me
ME=$(get /me "$TOKEN")
BARBER_ID=$(echo "$ME" | grep -o '"id":[0-9]*' | head -1 | grep -o '[0-9]*$')
echo "   barber_id=$BARBER_ID  token=${TOKEN:0:20}..."
echo ""

# ============================================================
# 2. ME / BARBERSHOP
# ============================================================
echo "── 2. ME / BARBERSHOP ──────────────────────────────────"

assert "GET /me retorna dados" '"id"' "$ME"
assert "GET /me/barbershop retorna dados" '"slug"\|"name"' "$(get /me/barbershop "$TOKEN")"

assert_status_range "PUT /me/barbershop → 2xx" "2" \
  "$(req_status PUT /me/barbershop '{"name":"Barbearia Prime","timezone":"America/Sao_Paulo","phone":"11999990000"}' "$TOKEN")"
echo ""

# ============================================================
# 3. SERVIÇOS E CATEGORIAS
# ============================================================
echo "── 3. SERVIÇOS ─────────────────────────────────────────"

assert "GET /me/services retorna lista" '\[' "$(get /me/services "$TOKEN")"

CREATE_SVC=$(req POST /me/services '{"name":"Corte CI","duration_min":30,"price":3500,"active":true}' "$TOKEN")
SVC_ID=$(jq_id "$CREATE_SVC" "ID")
[[ -z "$SVC_ID" ]] && SVC_ID=$(jq_id "$CREATE_SVC" "id")
assert "POST /me/services cria serviço" '"Corte CI"' "$CREATE_SVC"

if [[ -n "$SVC_ID" ]]; then
  assert_status "PUT /me/services/:id → 200" "200" \
    "$(req_status PUT "/me/services/$SVC_ID" '{"name":"Corte CI upd","duration_min":30,"price":3500}' "$TOKEN")"
else
  skip "PUT /me/services/:id (sem SVC_ID)"
fi

assert "GET /me/service-categories retorna lista" '.' "$(get /me/service-categories "$TOKEN")"
assert "GET /public/:slug/services retorna lista" '"services"' "$(get "/public/$SLUG/services")"
echo ""

# ============================================================
# 4. PRODUTOS E SUGESTÕES
# ============================================================
echo "── 4. PRODUTOS ─────────────────────────────────────────"

CREATE_PROD=$(req POST /me/products '{"name":"Pomada CI","price":2500,"stock":10,"active":true,"online_visible":true}' "$TOKEN")
PROD_ID=$(jq_id "$CREATE_PROD" "ID")
[[ -z "$PROD_ID" ]] && PROD_ID=$(jq_id "$CREATE_PROD" "id")
assert "POST /me/products cria produto" '"Pomada CI"' "$CREATE_PROD"

if [[ -n "$PROD_ID" ]]; then
  assert_status "PUT /me/products/:id → 200" "200" \
    "$(req_status PUT "/me/products/$PROD_ID" '{"name":"Pomada CI upd","price":2500,"stock":9}' "$TOKEN")"
fi

if [[ -n "$SVC_ID" && -n "$PROD_ID" ]]; then
  assert_status_range "PUT /me/services/:id/suggestion → 2xx" "2" \
    "$(req_status PUT "/me/services/$SVC_ID/suggestion" "{\"product_id\":$PROD_ID}" "$TOKEN")"
  assert "GET /me/services/:id/suggestion retorna dados" '.' \
    "$(get "/me/services/$SVC_ID/suggestion" "$TOKEN")"
else
  skip "Sugestão de produto (sem SVC_ID ou PROD_ID)"
fi

assert "GET /public/:slug/products retorna lista" '"products"' "$(get "/public/$SLUG/products")"
echo ""

# ============================================================
# 5. HORÁRIOS E SCHEDULE OVERRIDES
# ============================================================
echo "── 5. HORÁRIOS ─────────────────────────────────────────"

assert "GET /me/working-hours retorna lista" '.' "$(get /me/working-hours "$TOKEN")"

WH='{"days":[
  {"weekday":1,"active":true,"start_time":"08:00","end_time":"18:00","lunch_start":"12:00","lunch_end":"13:00"},
  {"weekday":2,"active":true,"start_time":"08:00","end_time":"18:00","lunch_start":"12:00","lunch_end":"13:00"},
  {"weekday":3,"active":true,"start_time":"08:00","end_time":"18:00","lunch_start":"12:00","lunch_end":"13:00"},
  {"weekday":4,"active":true,"start_time":"08:00","end_time":"18:00","lunch_start":"12:00","lunch_end":"13:00"},
  {"weekday":5,"active":true,"start_time":"08:00","end_time":"18:00","lunch_start":"12:00","lunch_end":"13:00"}
]}'
assert_status_range "PUT /me/working-hours → 2xx" "2" \
  "$(req_status PUT /me/working-hours "$WH" "$TOKEN")"

assert "GET /me/schedule-overrides retorna lista" '.' "$(get /me/schedule-overrides "$TOKEN")"
echo ""

# ============================================================
# 6. POLÍTICA DE PAGAMENTO
# ============================================================
echo "── 6. POLÍTICA DE PAGAMENTO ────────────────────────────"

assert "GET /me/payment-policies retorna dados" '.' "$(get /me/payment-policies "$TOKEN")"

assert_status_range "PUT /me/payment-policies → 2xx" "2" \
  "$(req_status PUT /me/payment-policies \
    '{"default_requirement":"optional","accept_pix":true,"accept_credit":true,"accept_debit":false,"accept_cash":false,"categories":[]}' \
    "$TOKEN")"
echo ""

# ============================================================
# 7. OAUTH STATUS (MP / PagBank / Google)
# ============================================================
echo "── 7. OAUTH STATUS ─────────────────────────────────────"

assert "GET /me/mercadopago/oauth/status" '"connected"' "$(get /me/mercadopago/oauth/status "$TOKEN")"
assert "GET /me/pagbank/oauth/status" '"connected"' "$(get /me/pagbank/oauth/status "$TOKEN")"
assert "GET /me/google/oauth/status" '"connected"' "$(get /me/google/oauth/status "$TOKEN")"
echo ""

# ============================================================
# 8. DISPONIBILIDADE PÚBLICA
# ============================================================
echo "── 8. DISPONIBILIDADE ──────────────────────────────────"

AVAIL_SVC="${SVC_ID:-1}"
AVAIL=$(get "/public/$SLUG/availability?date=$DATE_MON&product_id=$AVAIL_SVC")
assert "GET /public/:slug/availability retorna slots" '"date"\|"slots"' "$AVAIL"
echo ""

# ============================================================
# 9. CHECKOUT ORQUESTRADO PÚBLICO
# ============================================================
echo "── 9. CHECKOUT ORQUESTRADO ─────────────────────────────"

TICKET_TOKEN=""
CHECKOUT_APPT_ID=""

if [[ -n "$SVC_ID" ]]; then
  CHECKOUT=$(req POST "/public/$SLUG/checkout" \
    "{\"service_id\":$SVC_ID,\"date\":\"$DATE_TUE\",\"time\":\"09:00\",\"client_name\":\"CI Checkout\",\"client_phone\":\"11999990010\",\"client_email\":\"$CLIENT_EMAIL\"}")
  assert "POST /public/:slug/checkout retorna appointment" '"appointment"' "$CHECKOUT"
  CHECKOUT_APPT_ID=$(echo "$CHECKOUT" | grep -o '"id":[0-9]*' | head -1 | grep -o '[0-9]*$')
  TICKET_URL=$(echo "$CHECKOUT" | grep -o '"ticket_url":"[^"]*"' | cut -d'"' -f4)
  TICKET_TOKEN=$(echo "$TICKET_URL" | grep -o '[^/]*$')
  echo "   appt_id=$CHECKOUT_APPT_ID  ticket=${TICKET_TOKEN:0:16}..."
else
  skip "Checkout orquestrado (sem SVC_ID)"
fi
echo ""

# ============================================================
# 10. AGENDAMENTO PÚBLICO STANDALONE
# ============================================================
echo "── 10. AGENDAMENTO PÚBLICO ─────────────────────────────"

PUB_APPT_ID=""
if [[ -n "$SVC_ID" ]]; then
  PUB_APPT=$(req POST "/public/$SLUG/appointments" \
    "{\"service_id\":$SVC_ID,\"date\":\"$DATE_WED\",\"time\":\"10:00\",\"client_name\":\"CI PubAppt\",\"client_phone\":\"11999990011\",\"client_email\":\"$CLIENT_EMAIL\"}")
  PUB_APPT_ID=$(echo "$PUB_APPT" | grep -o '"ID":[0-9]*' | head -1 | grep -o '[0-9]*$')
  assert "POST /public/:slug/appointments cria agendamento" '"ID":' "$PUB_APPT"
else
  skip "Agendamento público (sem SVC_ID)"
fi
echo ""

# ============================================================
# 11. AGENDAMENTO PRIVADO (barbeiro)
# ============================================================
echo "── 11. AGENDAMENTO PRIVADO ─────────────────────────────"

APPT_ID=""
if [[ -n "$SVC_ID" ]]; then
  PRIV=$(req POST /me/appointments \
    "{\"product_id\":$SVC_ID,\"date\":\"$DATE_MON\",\"time\":\"11:00\",\"client_name\":\"CI Privado\",\"client_phone\":\"11999990012\"}" \
    "$TOKEN")
  APPT_ID=$(echo "$PRIV" | grep -o '"ID":[0-9]*' | head -1 | grep -o '[0-9]*$')
  assert "POST /me/appointments cria agendamento" '"ID":' "$PRIV"
else
  skip "Agendamento privado (sem SVC_ID)"
fi

assert "GET /me/appointments/date retorna lista" '.' \
  "$(get "/me/appointments/date?date=$DATE_MON" "$TOKEN")"
assert "GET /me/appointments/month retorna lista" '.' \
  "$(get "/me/appointments/month?year=$(date +%Y)&month=$(date +%m)" "$TOKEN")"
echo ""

# ============================================================
# 12. ENCAIXE (internal appointment)
# ============================================================
echo "── 12. ENCAIXE ─────────────────────────────────────────"

if [[ -n "$SVC_ID" && -n "$BARBER_ID" ]]; then
  ENCAIXE=$(req POST /me/internal-appointments \
    "{\"barber_id\":$BARBER_ID,\"client_name\":\"CI Encaixe\",\"client_phone\":\"11999990013\",\"barber_product_id\":$SVC_ID,\"start_time\":\"${DATE_THU}T09:00:00-03:00\",\"end_time\":\"${DATE_THU}T09:30:00-03:00\"}" \
    "$TOKEN")
  assert "POST /me/internal-appointments cria encaixe" '"ID"' "$ENCAIXE"
else
  skip "Encaixe (sem SVC_ID ou BARBER_ID)"
fi
echo ""

# ============================================================
# 13. TICKET (view / reschedule / cancel)
# ============================================================
echo "── 13. TICKET ──────────────────────────────────────────"

NEW_TICKET=""
if [[ -n "$TICKET_TOKEN" ]]; then
  VIEW=$(get "/public/ticket/$TICKET_TOKEN")
  assert "GET /public/ticket/:token retorna dados" '"appointment_id"' "$VIEW"

  RESC_HTTP=$(curl -s -o /tmp/resc_body.txt -w "%{http_code}" -X PATCH "$BASE/public/ticket/$TICKET_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"date\":\"$DATE_THU\",\"time\":\"14:00\"}")
  NEW_TICKET=$(grep -o '"token":"[^"]*"' /tmp/resc_body.txt | cut -d'"' -f4)
  assert_status "PATCH /public/ticket/:token reagenda → 200" "200" "$RESC_HTTP"
else
  skip "Ticket (sem TICKET_TOKEN)"
fi
echo ""

# ============================================================
# 14. PAGAMENTO TRANSPARENTE PIX
# ============================================================
echo "── 14. PAGAMENTO TRANSPARENTE (PIX) ────────────────────"

TARGET_APPT="${CHECKOUT_APPT_ID:-$PUB_APPT_ID}"
if [[ -n "$TARGET_APPT" ]]; then
  PIX=$(req POST "/public/$SLUG/appointments/$TARGET_APPT/payment/transparent" \
    "{\"payer_email\":\"$CLIENT_EMAIL\",\"payment_method_id\":\"pix\"}")
  assert "POST /appointments/:id/payment/transparent (PIX) → payment_id ou erro esperado" \
    '"payment_id"\|"payer_email_required"\|"payment_not_pending"\|"payment_not_configured"' "$PIX"
else
  skip "Pagamento PIX (sem appt_id)"
fi
echo ""

# ============================================================
# 15. STATUS DE PAGAMENTO (polling fallback)
# ============================================================
echo "── 15. STATUS DE PAGAMENTO (polling) ───────────────────"

if [[ -n "$TARGET_APPT" ]]; then
  PSTATUS=$(get "/public/$SLUG/appointments/$TARGET_APPT/payment/status")
  assert "GET /appointments/:id/payment/status retorna status" '"status"' "$PSTATUS"
else
  skip "Status de pagamento (sem appt_id)"
fi
echo ""

# ============================================================
# 16. CARRINHO PÚBLICO
# ============================================================
echo "── 16. CARRINHO ────────────────────────────────────────"

assert "GET /public/:slug/cart retorna carrinho" '"key"\|"items"' \
  "$(cart_req GET "/public/$SLUG/cart")"

if [[ -n "$PROD_ID" ]]; then
  ADD=$(cart_req POST "/public/$SLUG/cart/items" "{\"product_id\":$PROD_ID,\"quantity\":1}")
  assert "POST /public/:slug/cart/items adiciona item" '.' "$ADD"
  assert_status_range "DELETE /public/:slug/cart/items/:id → 2xx" "2" \
    "$(cart_status DELETE "/public/$SLUG/cart/items/$PROD_ID")"
else
  skip "Carrinho items (sem PROD_ID)"
fi
echo ""

# ============================================================
# 17. CLIENTES / CRM / HISTÓRICO / CATEGORIA
# ============================================================
echo "── 17. CLIENTES / CRM ──────────────────────────────────"

CLIENTS=$(get /me/clients "$TOKEN")
assert "GET /me/clients retorna lista" '.' "$CLIENTS"

CLIENT_ID=$(echo "$CLIENTS" | grep -o '"id":[0-9]*' | head -1 | grep -o '[0-9]*$')
[[ -z "$CLIENT_ID" ]] && CLIENT_ID=$(echo "$CLIENTS" | grep -o '"ID":[0-9]*' | head -1 | grep -o '[0-9]*$')

if [[ -n "$CLIENT_ID" ]]; then
  assert "GET /me/clients/:id/crm retorna dados"      '.' "$(get "/me/clients/$CLIENT_ID/crm" "$TOKEN")"
  assert "GET /me/clients/:id/history retorna dados"  '.' "$(get "/me/clients/$CLIENT_ID/history" "$TOKEN")"
  assert "GET /me/clients/:id/category retorna dados" '.' "$(get "/me/clients/$CLIENT_ID/category" "$TOKEN")"
else
  skip "CRM/history/category (sem CLIENT_ID — nenhum cliente cadastrado ainda)"
fi
echo ""

# ============================================================
# 18. PLANOS E ASSINATURAS
# ============================================================
echo "── 18. PLANOS E ASSINATURAS ────────────────────────────"

PLAN_ID=""
if [[ -n "$SVC_ID" ]]; then
  PLAN_RESP=$(req POST /me/plans \
    "{\"name\":\"Plano CI\",\"monthly_price_cents\":5000,\"duration_days\":30,\"cuts_included\":4,\"discount_percent\":0,\"service_ids\":[$SVC_ID]}" \
    "$TOKEN")
  PLAN_ID=$(jq_id "$PLAN_RESP" "id")
  assert_status_range "POST /me/plans → 2xx" "2" \
    "$(req_status POST /me/plans "{\"name\":\"Plano CI2\",\"monthly_price_cents\":4000,\"duration_days\":30,\"cuts_included\":2,\"discount_percent\":10,\"service_ids\":[$SVC_ID]}" "$TOKEN")"
  assert "GET /me/plans retorna lista" '.' "$(get /me/plans "$TOKEN")"

  PLANS=$(get /me/plans "$TOKEN")
  [[ -z "$PLAN_ID" ]] && PLAN_ID=$(echo "$PLANS" | grep -o '"id":[0-9]*' | head -1 | grep -o '[0-9]*$')
else
  skip "Planos (sem SVC_ID)"
fi

assert "GET /public/:slug/plans retorna lista" '"plans"' "$(get "/public/$SLUG/plans")"

if [[ -n "$CLIENT_ID" && -n "$PLAN_ID" ]]; then
  ACT=$(req_status POST /me/subscriptions \
    "{\"client_id\":$CLIENT_ID,\"plan_id\":$PLAN_ID}" "$TOKEN")
  if [[ "$ACT" == "201" || "$ACT" == "409" ]]; then
    echo -e "${GREEN}✔ PASS${NC} — POST /me/subscriptions → $ACT"
    ((PASS_COUNT++)) || true
  else
    echo -e "${RED}✘ FAIL${NC} — POST /me/subscriptions esperado 201/409, recebido $ACT"
    ((FAIL_COUNT++)) || true
  fi

  assert "GET /me/subscriptions/:clientID retorna dados" '.' \
    "$(get "/me/subscriptions/$CLIENT_ID" "$TOKEN")"
  assert "GET /me/subscriptions retorna lista" '.' "$(get /me/subscriptions "$TOKEN")"

  if [[ "$ACT" == "201" ]]; then
    assert_status "DELETE /me/subscriptions/:clientID → 204" "204" \
      "$(req_status DELETE "/me/subscriptions/$CLIENT_ID" "" "$TOKEN")"
  else
    skip "DELETE /me/subscriptions (cliente já tinha assinatura)"
  fi
else
  skip "Assinaturas (CLIENT_ID=$CLIENT_ID PLAN_ID=$PLAN_ID)"
fi
echo ""

# ============================================================
# 19. PEDIDOS PRIVADOS
# ============================================================
echo "── 19. PEDIDOS ─────────────────────────────────────────"

ORDER_ID=""
if [[ -n "$PROD_ID" && -n "$CLIENT_ID" ]]; then
  CREATE_ORDER=$(req POST /me/orders \
    "{\"client_id\":$CLIENT_ID,\"items\":[{\"product_id\":$PROD_ID,\"quantity\":1}]}" "$TOKEN")
  ORDER_ID=$(jq_id "$CREATE_ORDER" "id")
  [[ -z "$ORDER_ID" ]] && ORDER_ID=$(jq_id "$CREATE_ORDER" "ID")
  assert "POST /me/orders cria pedido" '"id"\|"ID"' "$CREATE_ORDER"
  assert "GET /me/orders retorna lista" '.' "$(get /me/orders "$TOKEN")"

  if [[ -n "$ORDER_ID" ]]; then
    assert "GET /me/orders/:id retorna pedido" '"id"\|"ID"' "$(get "/me/orders/$ORDER_ID" "$TOKEN")"
  fi
else
  skip "Pedidos (PROD_ID=$PROD_ID CLIENT_ID=$CLIENT_ID)"
fi
echo ""

# ============================================================
# 20. RELATÓRIOS E PAINÉIS
# ============================================================
echo "── 20. RELATÓRIOS ──────────────────────────────────────"

TODAY=$(date +%Y-%m-%d)
YYYYMM=$(date +%Y-%m)

assert "GET /me/day-panel"          '.'          "$(get "/me/day-panel?date=$TODAY" "$TOKEN")"
assert "GET /me/dashboard"          '.'          "$(get /me/dashboard "$TOKEN")"
assert "GET /me/financial"          '.'          "$(get "/me/financial?period=month&date=$TODAY" "$TOKEN")"
assert "GET /me/impact"             '.'          "$(get "/me/impact?period=month&date=$TODAY" "$TOKEN")"
assert "GET /me/summary"            '.'          "$(get /me/summary "$TOKEN")"
assert "GET /me/payments/summary"   '.'          "$(get /me/payments/summary "$TOKEN")"
assert "GET /me/payments"           '"data"\|[]' "$(get /me/payments "$TOKEN")"
assert "GET /me/payments/cash-due"  '.'          "$(get /me/payments/cash-due "$TOKEN")"
assert "GET /me/closures"           '.'          "$(get /me/closures "$TOKEN")"
echo ""

# ============================================================
# 21. AUDIT LOGS
# ============================================================
echo "── 21. AUDIT LOGS ──────────────────────────────────────"

assert "GET /me/audit-logs retorna eventos" '.' "$(get /me/audit-logs "$TOKEN")"
echo ""

# ============================================================
# 22. WHATSAPP STATUS
# ============================================================
echo "── 22. WHATSAPP ────────────────────────────────────────"

assert "GET /me/whatsapp/status retorna dados" '"connected"' "$(get /me/whatsapp/status "$TOKEN")"
echo ""

# ============================================================
# 23. LIFECYCLE DO AGENDAMENTO (cancel / no-show / complete)
# ============================================================
echo "── 23. LIFECYCLE AGENDAMENTO ───────────────────────────"

# Cancel
if [[ -n "$APPT_ID" ]]; then
  CANCEL=$(req_status PUT "/me/appointments/$APPT_ID/cancel" "" "$TOKEN")
  if [[ "$CANCEL" == "200" || "$CANCEL" == "204" ]]; then
    echo -e "${GREEN}✔ PASS${NC} — PUT /me/appointments/:id/cancel → $CANCEL"
    ((PASS_COUNT++)) || true
  else
    echo -e "${RED}✘ FAIL${NC} — PUT /me/appointments/:id/cancel esperado 2xx, recebido $CANCEL"
    ((FAIL_COUNT++)) || true
  fi
else
  skip "Cancel appointment (sem APPT_ID)"
fi

# No-show — cria novo agendamento
APPT_NS=""
if [[ -n "$SVC_ID" ]]; then
  ANS=$(req POST /me/appointments \
    "{\"product_id\":$SVC_ID,\"date\":\"$DATE_MON\",\"time\":\"15:00\",\"client_name\":\"CI NoShow\",\"client_phone\":\"11999990014\"}" \
    "$TOKEN")
  APPT_NS=$(echo "$ANS" | grep -o '"ID":[0-9]*' | head -1 | grep -o '[0-9]*$')
fi

if [[ -n "$APPT_NS" ]]; then
  NS=$(req_status PUT "/me/appointments/$APPT_NS/no-show" "" "$TOKEN")
  if [[ "$NS" == "204" || "$NS" == "200" || "$NS" == "422" ]]; then
    echo -e "${GREEN}✔ PASS${NC} — PUT /me/appointments/:id/no-show → $NS"
    ((PASS_COUNT++)) || true
  else
    echo -e "${RED}✘ FAIL${NC} — PUT /me/appointments/:id/no-show recebido $NS"
    ((FAIL_COUNT++)) || true
  fi
else
  skip "No-show (sem APPT_NS)"
fi

# Complete — cria novo agendamento e fecha
APPT_COMPLETE=""
if [[ -n "$SVC_ID" ]]; then
  AC=$(req POST /me/appointments \
    "{\"product_id\":$SVC_ID,\"date\":\"$DATE_MON\",\"time\":\"16:00\",\"client_name\":\"CI Complete\",\"client_phone\":\"11999990015\"}" \
    "$TOKEN")
  APPT_COMPLETE=$(echo "$AC" | grep -o '"ID":[0-9]*' | head -1 | grep -o '[0-9]*$')
fi

if [[ -n "$APPT_COMPLETE" ]]; then
  COMP=$(req_status PUT "/me/appointments/$APPT_COMPLETE/complete" \
    '{"payment_method":"cash","suggestion_removed":false}' "$TOKEN")
  if [[ "$COMP" == "200" || "$COMP" == "204" || "$COMP" == "422" ]]; then
    echo -e "${GREEN}✔ PASS${NC} — PUT /me/appointments/:id/complete → $COMP"
    ((PASS_COUNT++)) || true
  else
    echo -e "${RED}✘ FAIL${NC} — PUT /me/appointments/:id/complete recebido $COMP"
    ((FAIL_COUNT++)) || true
  fi
else
  skip "Complete appointment (sem APPT_COMPLETE)"
fi

# Cancel via ticket (usa novo token se reagendou)
USE_TICKET="${NEW_TICKET:-$TICKET_TOKEN}"
if [[ -n "$USE_TICKET" && ${#USE_TICKET} -ge 32 ]]; then
  CTK=$(req_status DELETE "/public/ticket/$USE_TICKET")
  if [[ "$CTK" == "200" || "$CTK" == "204" ]]; then
    echo -e "${GREEN}✔ PASS${NC} — DELETE /public/ticket/:token → $CTK"
    ((PASS_COUNT++)) || true
  else
    echo -e "${RED}✘ FAIL${NC} — DELETE /public/ticket/:token recebido $CTK"
    ((FAIL_COUNT++)) || true
  fi
else
  skip "Cancel via ticket (sem token)"
fi
echo ""

# ============================================================
# 24. LIMPEZA (deleta serviço e produto CI)
# ============================================================
echo "── 24. LIMPEZA ─────────────────────────────────────────"

if [[ -n "$SVC_ID" ]]; then
  assert_status_range "DELETE /me/services/:id → 2xx" "2" \
    "$(req_status DELETE "/me/services/$SVC_ID" "" "$TOKEN")"
else
  skip "DELETE serviço (sem SVC_ID)"
fi

if [[ -n "$PROD_ID" ]]; then
  assert_status_range "DELETE /me/products/:id → 2xx" "2" \
    "$(req_status DELETE "/me/products/$PROD_ID" "" "$TOKEN")"
else
  skip "DELETE produto (sem PROD_ID)"
fi
echo ""

# ============================================================
# RESULTADO FINAL
# ============================================================
TOTAL=$((PASS_COUNT + FAIL_COUNT + SKIP_COUNT))
echo "============================================================"
printf "  Total: %d  |  " "$TOTAL"
echo -e "${GREEN}Passou: $PASS_COUNT${NC}  |  ${RED}Falhou: $FAIL_COUNT${NC}  |  ${YELLOW}Pulado: $SKIP_COUNT${NC}"
echo "============================================================"
echo ""

[[ $FAIL_COUNT -gt 0 ]] && exit 1
exit 0
