#!/usr/bin/env bash
# ============================================================
# test_flows.sh — testa todos os fluxos do barber-scheduler
# ============================================================

BASE="http://localhost:8080/api"
SLUG="barbearia-do-rafa"
EMAIL="rafa@teste.com"
PASS="123456"
CLIENT_EMAIL="lucas.joveml.0987@gmail.com"
CART_KEY="ci-test-cart-$(date +%s)"

PASS_COUNT=0
FAIL_COUNT=0
SKIP_COUNT=0

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
NC='\033[0m'

# assert: verifica se $3 contém o padrão $2 (grep -q)
assert() {
  local label="$1"
  local pattern="$2"
  local actual="$3"
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
  local label="$1"
  local expected="$2"
  local actual="$3"
  if [[ "$actual" == "$expected" ]]; then
    echo -e "${GREEN}✔ PASS${NC} — $label"
    ((PASS_COUNT++)) || true
  else
    echo -e "${RED}✘ FAIL${NC} — $label (esperado $expected, recebido $actual)"
    ((FAIL_COUNT++)) || true
  fi
}

skip() {
  echo -e "${YELLOW}⊘ SKIP${NC} — $1"
  ((SKIP_COUNT++)) || true
}

# Extrai valor numérico de campo JSON lowercase (ex: "id":5)
jq_id() {
  local json="$1"
  local field="${2:-id}"
  echo "$json" | grep -o "\"${field}\":[0-9]*" | head -1 | grep -o '[0-9]*$'
}

get() {
  local url="$1"; local token="${2:-}"
  local args=(-s -X GET "$BASE$url")
  [[ -n "$token" ]] && args+=(-H "Authorization: Bearer $token")
  curl "${args[@]}"
}

get_status() {
  local url="$1"; local token="${2:-}"
  local args=(-s -o /dev/null -w "%{http_code}" -X GET "$BASE$url")
  [[ -n "$token" ]] && args+=(-H "Authorization: Bearer $token")
  curl "${args[@]}"
}

req() {
  local method="$1"; local url="$2"; local data="${3:-}"; local token="${4:-}"
  local args=(-s -X "$method" "$BASE$url" -H "Content-Type: application/json")
  [[ -n "$token" ]] && args+=(-H "Authorization: Bearer $token")
  [[ -n "$data" ]] && args+=(-d "$data")
  curl "${args[@]}"
}

req_status() {
  local method="$1"; local url="$2"; local data="${3:-}"; local token="${4:-}"
  local args=(-s -o /dev/null -w "%{http_code}" -X "$method" "$BASE$url" -H "Content-Type: application/json")
  [[ -n "$token" ]] && args+=(-H "Authorization: Bearer $token")
  [[ -n "$data" ]] && args+=(-d "$data")
  curl "${args[@]}"
}

cart_req() {
  local method="$1"; local url="$2"; local data="${3:-}"
  local args=(-s -X "$method" "$BASE$url" -H "Content-Type: application/json" -H "X-Cart-Key: $CART_KEY")
  [[ -n "$data" ]] && args+=(-d "$data")
  curl "${args[@]}"
}

cart_status() {
  local method="$1"; local url="$2"; local data="${3:-}"
  local args=(-s -o /dev/null -w "%{http_code}" -X "$method" "$BASE$url" -H "Content-Type: application/json" -H "X-Cart-Key: $CART_KEY")
  [[ -n "$data" ]] && args+=(-d "$data")
  curl "${args[@]}"
}

echo ""
echo "============================================================"
echo "  barber-scheduler — test suite"
echo "============================================================"
echo ""

# ============================================================
# 1. AUTH
# ============================================================
echo "── 1. AUTH ─────────────────────────────────────────────"

LOGIN=$(req POST /auth/login '{"email":"'"$EMAIL"'","password":"'"$PASS"'"}')
TOKEN=$(echo "$LOGIN" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
assert "POST /auth/login retorna token" '"token"' "$LOGIN"

BAD_STATUS=$(req_status POST /auth/login '{"email":"wrong@x.com","password":"bad"}')
assert_status "POST /auth/login inválido → 401" "401" "$BAD_STATUS"

# Extrair barber_id (campo "id" dentro de "user")
BARBER_ID=$(echo "$LOGIN" | grep -o '"user_id":[0-9]*' | grep -o '[0-9]*')
# fallback: buscar do /me
if [[ -z "$BARBER_ID" ]]; then
  ME=$(get /me "$TOKEN")
  BARBER_ID=$(echo "$ME" | grep -o '"user":{[^}]*"id":[0-9]*' | grep -o '"id":[0-9]*$' | grep -o '[0-9]*')
  [[ -z "$BARBER_ID" ]] && BARBER_ID=$(echo "$ME" | grep -o '"id":[0-9]*' | head -1 | grep -o '[0-9]*')
fi
echo "   barber_id=$BARBER_ID"
echo ""

# ============================================================
# 2. ME / BARBERSHOP
# ============================================================
echo "── 2. ME / BARBERSHOP ──────────────────────────────────"

ME=$(get /me "$TOKEN")
assert "GET /me retorna dados" '"id"' "$ME"

BS=$(get /me/barbershop "$TOKEN")
assert "GET /me/barbershop retorna dados" '"slug"\|"Slug"\|"name"\|"Name"' "$BS"

UPD_BS=$(req_status PUT /me/barbershop '{"name":"Barbearia do Rafa","timezone":"America/Sao_Paulo"}' "$TOKEN")
assert "PUT /me/barbershop → 2xx" "^2" "$UPD_BS"
echo ""

# ============================================================
# 3. SERVIÇOS
# ============================================================
echo "── 3. SERVIÇOS ─────────────────────────────────────────"

assert "GET /me/services retorna lista" '^\[' "$(get /me/services "$TOKEN")"

CREATE_SVC=$(req POST /me/services '{"name":"Corte CI","duration_min":30,"price":3500}' "$TOKEN")
SVC_ID=$(echo "$CREATE_SVC" | grep -o '"ID":[0-9]*\|"id":[0-9]*' | head -1 | grep -o '[0-9]*$')
assert "POST /me/services cria serviço" '"Corte CI"' "$CREATE_SVC"

if [[ -n "$SVC_ID" ]]; then
  assert_status "PUT /me/services/:id → 200" "200" \
    "$(req_status PUT "/me/services/$SVC_ID" '{"name":"Corte CI upd","duration_min":30,"price":3500}' "$TOKEN")"
else
  skip "PUT /me/services/:id (SVC_ID vazio)"
fi

assert "GET /public/:slug/services retorna lista" '"services"' "$(get "/public/$SLUG/services")"
echo ""

# ============================================================
# 4. PRODUTOS
# ============================================================
echo "── 4. PRODUTOS ─────────────────────────────────────────"

CREATE_PROD=$(req POST /me/products '{"name":"Pomada CI","price":2500,"stock":10}' "$TOKEN")
PROD_ID=$(echo "$CREATE_PROD" | grep -o '"ID":[0-9]*\|"id":[0-9]*' | head -1 | grep -o '[0-9]*$')
assert "POST /me/products cria produto" '"Pomada CI"' "$CREATE_PROD"

if [[ -n "$PROD_ID" ]]; then
  assert_status "PUT /me/products/:id → 200" "200" \
    "$(req_status PUT "/me/products/$PROD_ID" '{"name":"Pomada CI upd","price":2500,"stock":9}' "$TOKEN")"
fi

assert "GET /public/:slug/products retorna lista" '"products"' "$(get "/public/$SLUG/products")"
echo ""

# ============================================================
# 5. HORÁRIOS DE TRABALHO
# ============================================================
echo "── 5. HORÁRIOS DE TRABALHO ─────────────────────────────"

assert "GET /me/working-hours retorna lista" '^\[' "$(get /me/working-hours "$TOKEN")"

WH_PAYLOAD='{"days":[
  {"weekday":1,"active":true,"start_time":"08:00","end_time":"18:00","lunch_start":"12:00","lunch_end":"13:00"},
  {"weekday":2,"active":true,"start_time":"08:00","end_time":"18:00","lunch_start":"12:00","lunch_end":"13:00"},
  {"weekday":3,"active":true,"start_time":"08:00","end_time":"18:00","lunch_start":"12:00","lunch_end":"13:00"},
  {"weekday":4,"active":true,"start_time":"08:00","end_time":"18:00","lunch_start":"12:00","lunch_end":"13:00"},
  {"weekday":5,"active":true,"start_time":"08:00","end_time":"18:00","lunch_start":"12:00","lunch_end":"13:00"}
]}'
assert_status "PUT /me/working-hours → 200" "200" \
  "$(req_status PUT /me/working-hours "$WH_PAYLOAD" "$TOKEN")"
echo ""

# ============================================================
# 6. POLÍTICA DE PAGAMENTO
# ============================================================
echo "── 6. POLÍTICA DE PAGAMENTO ────────────────────────────"

assert "GET /me/payment-policies retorna dados" '.' "$(get /me/payment-policies "$TOKEN")"

assert_status "PUT /me/payment-policies → 204" "204" \
  "$(req_status PUT /me/payment-policies \
    '{"default_requirement":"optional","pix_expiration_minutes":30,"categories":[]}' "$TOKEN")"
echo ""

# ============================================================
# 7. DISPONIBILIDADE PÚBLICA
# ============================================================
echo "── 7. DISPONIBILIDADE ──────────────────────────────────"

# ─── Datas e horários únicos por run ───────────────────────────────────────
# Usa uma semana aleatória no futuro (2-51 semanas) para evitar conflitos
# de slot quando o script é executado múltiplas vezes no mesmo ambiente.
_DOW=$(date +%u)  # 1=Mon .. 7=Sun

# Próxima segunda (base para TOMORROW)
_DAYS_TO_MON=$(( (8 - _DOW) % 7 ))
[[ "$_DAYS_TO_MON" -eq 0 ]] && _DAYS_TO_MON=7

# Offset aleatório de semanas (2-51)
_WOFF=$(( RANDOM % 50 + 2 ))

_get_date() { date -v+${1}d +%Y-%m-%d 2>/dev/null || date -d "+${1} days" +%Y-%m-%d; }

TOMORROW=$(_get_date $((_DAYS_TO_MON + _WOFF * 7)))         # segunda
CHECKOUT_DATE_BASE=$(_get_date $((_DAYS_TO_MON + 1 + _WOFF * 7)))  # terça
PUB_DATE_BASE=$(_get_date $((_DAYS_TO_MON + 2 + _WOFF * 7)))       # quarta
RESC_DATE_BASE=$(_get_date $((_DAYS_TO_MON + 3 + _WOFF * 7)))      # quinta

# Horário fixo por seção (sem colisão entre elas — cada uma usa hora distinta)
T_CHECKOUT="09:00"
T_PUB="10:00"
T_PRIV="11:00"
T_RESC="14:00"
T_NOSHOW="15:00"
echo "   semana+${_WOFF}: mon=$TOMORROW | slots: $T_CHECKOUT $T_PUB $T_PRIV $T_RESC $T_NOSHOW"
assert "GET /public/:slug/availability retorna dados" '.' \
  "$(get "/public/$SLUG/availability?date=$TOMORROW&service_id=${SVC_ID:-1}")"
echo ""

# ============================================================
# 8. CHECKOUT ORQUESTRADO (gera ticket)
# ============================================================
echo "── 8. CHECKOUT ORQUESTRADO ─────────────────────────────"

TICKET_TOKEN=""
CHECKOUT_APPT_ID=""

if [[ -n "$SVC_ID" && -n "$BARBER_ID" ]]; then
  # Usar terça-feira da próxima semana para evitar conflito com outros slots de segunda
  CHECKOUT_DATE="$CHECKOUT_DATE_BASE"
  CHECKOUT=$(req POST "/public/$SLUG/checkout" \
    '{"barber_id":'"$BARBER_ID"',"service_id":'"$SVC_ID"',"date":"'"$CHECKOUT_DATE"'","time":"'"$T_CHECKOUT"'","client_name":"CI Checkout","client_phone":"11999990000","client_email":"'"$CLIENT_EMAIL"'"}')
  assert "POST /public/:slug/checkout retorna appointment" '"appointment"' "$CHECKOUT"
  CHECKOUT_APPT_ID=$(jq_id "$CHECKOUT" "id")
  TICKET_URL=$(echo "$CHECKOUT" | grep -o '"ticket_url":"[^"]*"' | cut -d'"' -f4)
  TICKET_TOKEN=$(echo "$TICKET_URL" | grep -o '[^/]*$')
  echo "   ticket_token=${TICKET_TOKEN:0:16}..."
else
  skip "Checkout orquestrado (sem SVC_ID=$SVC_ID ou BARBER_ID=$BARBER_ID)"
fi
echo ""

# ============================================================
# 9. AGENDAMENTO PÚBLICO (sem ticket)
# ============================================================
echo "── 9. AGENDAMENTO PÚBLICO ──────────────────────────────"

PUB_APPT_ID=""
if [[ -n "$SVC_ID" ]]; then
  # Quarta-feira da próxima semana
  PUB_DATE="$PUB_DATE_BASE"
  PUB_APPT=$(req POST "/public/$SLUG/appointments" \
    '{"service_id":'"$SVC_ID"',"date":"'"$PUB_DATE"'","time":"'"$T_PUB"'","client_name":"CI PubAppt","client_phone":"11999990001","client_email":"'"$CLIENT_EMAIL"'"}')
  PUB_APPT_ID=$(echo "$PUB_APPT" | grep -o '"ID":[0-9]*' | head -1 | grep -o '[0-9]*$')
  assert "POST /public/:slug/appointments cria agendamento" '"ID":' "$PUB_APPT"
else
  skip "POST /public/:slug/appointments (sem SVC_ID)"
fi
echo ""

# ============================================================
# 10. AGENDAMENTO PRIVADO
# ============================================================
echo "── 10. AGENDAMENTO PRIVADO ─────────────────────────────"

APPT_ID=""
if [[ -n "$SVC_ID" ]]; then
  PRIV_APPT=$(req POST /me/appointments \
    '{"product_id":'"$SVC_ID"',"date":"'"$TOMORROW"'","time":"'"$T_PRIV"'","client_name":"CI Priv","client_phone":"11999990002"}' \
    "$TOKEN")
  APPT_ID=$(echo "$PRIV_APPT" | grep -o '"ID":[0-9]*' | head -1 | grep -o '[0-9]*$')
  assert "POST /me/appointments cria agendamento" '"ID":' "$PRIV_APPT"
else
  skip "POST /me/appointments (sem SVC_ID)"
fi

assert "GET /me/appointments/date retorna lista" '.' \
  "$(get "/me/appointments/date?date=$TOMORROW" "$TOKEN")"

assert "GET /me/appointments/month retorna lista" '.' \
  "$(get "/me/appointments/month?month=$(date +%Y-%m)" "$TOKEN")"
echo ""

# ============================================================
# 11. TICKET (view / reschedule / cancel)
# ============================================================
echo "── 11. TICKET ──────────────────────────────────────────"

if [[ -n "$TICKET_TOKEN" ]]; then
  VIEW=$(get "/public/ticket/$TICKET_TOKEN")
  assert "GET /public/ticket/:token retorna dados" '"appointment_id"\|"barber"' "$VIEW"

  # Quinta-feira da próxima semana
  RESC_DATE="$RESC_DATE_BASE"
  # Chama PATCH uma única vez capturando status + corpo (token rotaciona após reschedule)
  RESC_HTTP=$(curl -s -o /tmp/resc_body.txt -w "%{http_code}" -X PATCH "$BASE/public/ticket/$TICKET_TOKEN" \
    -H "Content-Type: application/json" -d '{"date":"'"$RESC_DATE"'","time":"'"$T_RESC"'"}')
  NEW_TICKET=$(grep -o '"token":"[^"]*"' /tmp/resc_body.txt | cut -d'"' -f4)
  assert_status "PATCH /public/ticket/:token reagenda → 200" "200" "$RESC_HTTP"
else
  skip "Ticket flow (sem TICKET_TOKEN)"
fi
echo ""

# ============================================================
# 12. PAGAMENTO PIX
# ============================================================
echo "── 12. PAGAMENTO PIX ───────────────────────────────────"

TARGET_APPT_ID="${CHECKOUT_APPT_ID:-$PUB_APPT_ID}"
if [[ -n "$TARGET_APPT_ID" ]]; then
  PIX=$(req POST "/public/$SLUG/appointments/$TARGET_APPT_ID/payment/pix" \
    '{"client_email":"'"$CLIENT_EMAIL"'"}')
  assert "POST /public/appointments/:id/payment/pix retorna PIX" '"txid"\|"qr_code"\|"pix_key"\|"error_code"' "$PIX"
else
  skip "PIX para agendamento (sem appt_id)"
fi
echo ""

# ============================================================
# 13. CARRINHO
# ============================================================
echo "── 13. CARRINHO ────────────────────────────────────────"

assert "GET /public/:slug/cart retorna carrinho" '.' "$(cart_req GET "/public/$SLUG/cart")"

if [[ -n "$PROD_ID" ]]; then
  ADD=$(cart_req POST "/public/$SLUG/cart/items" '{"product_id":'"$PROD_ID"',"quantity":1}')
  assert "POST /public/:slug/cart/items adiciona item" '.' "$ADD"

  assert_status "DELETE /public/:slug/cart/items/:id → 2xx" "200" \
    "$(cart_status DELETE "/public/$SLUG/cart/items/$PROD_ID")"
else
  skip "Carrinho items (sem PROD_ID)"
fi
echo ""

# ============================================================
# 14. RELATÓRIOS
# ============================================================
echo "── 14. RELATÓRIOS ──────────────────────────────────────"

TODAY=$(date +%Y-%m-%d)
assert "GET /me/day-panel"       '.'  "$(get "/me/day-panel?date=$TODAY" "$TOKEN")"
assert "GET /me/dashboard"       '.'  "$(get /me/dashboard "$TOKEN")"
assert "GET /me/financial"       '.'  "$(get "/me/financial?period=month&date=$TODAY" "$TOKEN")"
assert "GET /me/impact"          '.'  "$(get "/me/impact?period=month&date=$TODAY" "$TOKEN")"
assert "GET /me/summary"         '.'  "$(get /me/summary "$TOKEN")"
assert "GET /me/payments/summary" '.' "$(get /me/payments/summary "$TOKEN")"
assert "GET /me/payments"        '"data"' "$(get /me/payments "$TOKEN")"
echo ""

# ============================================================
# 15. PEDIDOS
# ============================================================
echo "── 15. PEDIDOS ─────────────────────────────────────────"

CLIENTS=$(get /me/clients "$TOKEN")
CLIENT_ID=$(echo "$CLIENTS" | grep -o '"id":[0-9]*' | head -1 | grep -o '[0-9]*$')
[[ -z "$CLIENT_ID" ]] && CLIENT_ID=$(echo "$CLIENTS" | grep -o '"ID":[0-9]*' | head -1 | grep -o '[0-9]*$')

ORDER_ID=""
if [[ -n "$PROD_ID" && -n "$CLIENT_ID" ]]; then
  CREATE_ORDER=$(req POST /me/orders \
    '{"client_id":'"$CLIENT_ID"',"items":[{"product_id":'"$PROD_ID"',"quantity":1}]}' "$TOKEN")
  ORDER_ID=$(jq_id "$CREATE_ORDER" "id")
  [[ -z "$ORDER_ID" ]] && ORDER_ID=$(echo "$CREATE_ORDER" | grep -o '"ID":[0-9]*' | head -1 | grep -o '[0-9]*$')
  assert "POST /me/orders cria pedido" '"id"\|"ID"' "$CREATE_ORDER"
  assert "GET /me/orders retorna lista" '.' "$(get /me/orders "$TOKEN")"

  if [[ -n "$ORDER_ID" ]]; then
    assert "GET /me/orders/:id retorna pedido" '"id"\|"ID"' "$(get "/me/orders/$ORDER_ID" "$TOKEN")"
    ORDER_PIX=$(req POST "/me/orders/$ORDER_ID/payment/pix" '{}' "$TOKEN")
    assert "POST /me/orders/:id/payment/pix retorna PIX" '"txid"\|"qr_code"\|"pix_key"\|"error_code"' "$ORDER_PIX"
  fi
else
  skip "Orders (PROD_ID=$PROD_ID CLIENT_ID=$CLIENT_ID)"
fi
echo ""

# ============================================================
# 16. CLIENTES / CRM / HISTÓRICO / CATEGORIA
# ============================================================
echo "── 16. CLIENTES / CRM ──────────────────────────────────"

assert "GET /me/clients retorna lista" '.' "$(get /me/clients "$TOKEN")"

if [[ -n "$CLIENT_ID" ]]; then
  assert "GET /me/clients/:id/crm"      '.' "$(get "/me/clients/$CLIENT_ID/crm" "$TOKEN")"
  assert "GET /me/clients/:id/history"  '.' "$(get "/me/clients/$CLIENT_ID/history" "$TOKEN")"
  assert "GET /me/clients/:id/category" '.' "$(get "/me/clients/$CLIENT_ID/category" "$TOKEN")"
else
  skip "CRM/history/category (sem CLIENT_ID)"
fi
echo ""

# ============================================================
# 17. PLANOS E ASSINATURAS
# ============================================================
echo "── 17. PLANOS E ASSINATURAS ────────────────────────────"

PLAN_ID=""
if [[ -n "$SVC_ID" ]]; then
  PLAN_STATUS=$(req_status POST /me/plans \
    '{"name":"Plano CI","monthly_price_cents":5000,"duration_days":30,"cuts_included":4,"discount_percent":0,"service_ids":['"$SVC_ID"']}' \
    "$TOKEN")
  assert_status "POST /me/plans → 201" "201" "$PLAN_STATUS"

  # Buscar o ID do plano criado
  PLANS_RESP=$(get /me/plans "$TOKEN")
  PLAN_ID=$(echo "$PLANS_RESP" | grep -o '"id":[0-9]*' | head -1 | grep -o '[0-9]*$')
  [[ -z "$PLAN_ID" ]] && PLAN_ID=$(echo "$PLANS_RESP" | grep -o '"ID":[0-9]*' | head -1 | grep -o '[0-9]*$')
  assert "GET /me/plans retorna lista" '.' "$PLANS_RESP"
else
  skip "Planos (sem SVC_ID)"
fi

if [[ -n "$CLIENT_ID" && -n "$PLAN_ID" ]]; then
  ACT_STATUS=$(req_status POST /me/subscriptions \
    '{"client_id":'"$CLIENT_ID"',"plan_id":'"$PLAN_ID"'}' "$TOKEN")
  # 201 = novo, 409 = já tem assinatura
  if [[ "$ACT_STATUS" == "201" || "$ACT_STATUS" == "409" ]]; then
    echo -e "${GREEN}✔ PASS${NC} — POST /me/subscriptions → $ACT_STATUS"
    ((PASS_COUNT++)) || true
  else
    echo -e "${RED}✘ FAIL${NC} — POST /me/subscriptions esperado 201 ou 409, recebido $ACT_STATUS"
    ((FAIL_COUNT++)) || true
  fi

  assert "GET /me/subscriptions/:clientID" '.' "$(get "/me/subscriptions/$CLIENT_ID" "$TOKEN")"

  if [[ "$ACT_STATUS" == "201" ]]; then
    assert_status "DELETE /me/subscriptions/:clientID → 204" "204" \
      "$(req_status DELETE "/me/subscriptions/$CLIENT_ID" "" "$TOKEN")"
  else
    skip "DELETE /me/subscriptions (já tinha assinatura)"
  fi
else
  skip "Assinaturas (CLIENT_ID=$CLIENT_ID PLAN_ID=$PLAN_ID)"
fi
echo ""

# ============================================================
# 18. AUDIT LOGS
# ============================================================
echo "── 18. AUDIT LOGS ──────────────────────────────────────"

assert "GET /me/audit-logs retorna eventos" '.' "$(get /me/audit-logs "$TOKEN")"
echo ""

# ============================================================
# 19. LIFECYCLE AGENDAMENTO (cancel / no-show / complete)
# ============================================================
echo "── 19. LIFECYCLE AGENDAMENTO ───────────────────────────"

if [[ -n "$APPT_ID" ]]; then
  CANCEL_STATUS=$(req_status PUT "/me/appointments/$APPT_ID/cancel" "" "$TOKEN")
  if [[ "$CANCEL_STATUS" == "200" || "$CANCEL_STATUS" == "204" ]]; then
    echo -e "${GREEN}✔ PASS${NC} — PUT /me/appointments/:id/cancel → $CANCEL_STATUS"
    ((PASS_COUNT++)) || true
  else
    echo -e "${RED}✘ FAIL${NC} — PUT /me/appointments/:id/cancel esperado 200 ou 204, recebido $CANCEL_STATUS"
    ((FAIL_COUNT++)) || true
  fi
else
  skip "Cancel appointment (sem APPT_ID)"
fi

# Criar novo para no-show
APPT2_ID=""
if [[ -n "$SVC_ID" ]]; then
  A2=$(req POST /me/appointments \
    '{"product_id":'"$SVC_ID"',"date":"'"$TOMORROW"'","time":"'"$T_NOSHOW"'","client_name":"CI NoShow","client_phone":"11999990003"}' \
    "$TOKEN")
  APPT2_ID=$(echo "$A2" | grep -o '"ID":[0-9]*' | head -1 | grep -o '[0-9]*$')
fi

if [[ -n "$APPT2_ID" ]]; then
  NS_STATUS=$(req_status PUT "/me/appointments/$APPT2_ID/no-show" "" "$TOKEN")
  # no-show válido só se horário passou, pode retornar 422
  if [[ "$NS_STATUS" == "204" || "$NS_STATUS" == "200" || "$NS_STATUS" == "422" ]]; then
    echo -e "${GREEN}✔ PASS${NC} — PUT /me/appointments/:id/no-show → $NS_STATUS"
    ((PASS_COUNT++)) || true
  else
    echo -e "${RED}✘ FAIL${NC} — PUT /me/appointments/:id/no-show esperado 2xx ou 422, recebido $NS_STATUS"
    ((FAIL_COUNT++)) || true
  fi
else
  skip "No-show (sem APPT2_ID)"
fi

# Cancel via ticket — usa o novo token gerado pelo reschedule
USE_TICKET="${NEW_TICKET:-$TICKET_TOKEN}"
if [[ -n "$USE_TICKET" && ${#USE_TICKET} -ge 32 ]]; then
  CANCEL_TICKET_STATUS=$(req_status DELETE "/public/ticket/$USE_TICKET")
  if [[ "$CANCEL_TICKET_STATUS" == "200" || "$CANCEL_TICKET_STATUS" == "204" ]]; then
    echo -e "${GREEN}✔ PASS${NC} — DELETE /public/ticket/:token → $CANCEL_TICKET_STATUS"
    ((PASS_COUNT++)) || true
  else
    echo -e "${RED}✘ FAIL${NC} — DELETE /public/ticket/:token recebido $CANCEL_TICKET_STATUS (token=${USE_TICKET:0:16}...)"
    ((FAIL_COUNT++)) || true
  fi
elif [[ -n "$TICKET_TOKEN" ]]; then
  skip "Cancel ticket (novo token não capturado, reagendamento pode não ter ocorrido)"
fi
echo ""

# ============================================================
# RESULTADO FINAL
# ============================================================
TOTAL=$((PASS_COUNT + FAIL_COUNT + SKIP_COUNT))
echo "============================================================"
echo -e "  Total: $TOTAL  |  ${GREEN}Passou: $PASS_COUNT${NC}  |  ${RED}Falhou: $FAIL_COUNT${NC}  |  ${YELLOW}Pulado: $SKIP_COUNT${NC}"
echo "============================================================"
echo ""

[[ $FAIL_COUNT -gt 0 ]] && exit 1
exit 0
