// cmd/e2e/wave_test.go
//
// Onda de testes focada em velocidade, lógica e fluxo de agendamentos pagos.
//
// Pré-requisitos:
//   1. Banco com seed: go run ./cmd/seed
//   2. Servidor rodando: go run ./cmd/api  (MP_PROVIDER=mock, padrão)
//
// Execução:
//   BASE_URL=http://localhost:8080 go test ./cmd/e2e/... -run TestWave -v -timeout 120s

package e2e_test

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// ============================================================
// HELPERS DE TIMING E SETUP
// ============================================================

func measure(t *testing.T, label string, fn func() int) time.Duration {
	t.Helper()
	start := time.Now()
	code := fn()
	elapsed := time.Since(start)

	marker := "✓"
	if code == 0 {
		marker = "✗ (sem resposta)"
	} else if code >= 500 {
		marker = fmt.Sprintf("✗ HTTP %d", code)
	} else if code >= 400 {
		marker = fmt.Sprintf("⚠ HTTP %d", code)
	}
	t.Logf("  %s %-52s %5dms", marker, label, elapsed.Milliseconds())
	return elapsed
}

// waveSetup retorna o barber_id do usuário autenticado e o primeiro serviço disponível.
func waveSetup(t *testing.T) (barberID uint, svcID uint, svcName string) {
	t.Helper()

	// Barber ID vem do /api/me (user.id)
	var me map[string]any
	get("/api/me", &me)
	if user, ok := me["user"].(map[string]any); ok {
		barberID = id(user["id"])
	}

	// Serviço: GET /api/me/services retorna campos capitalizados (sem JSON tags no model)
	var services []map[string]any
	get("/api/me/services", &services)
	for _, svc := range services {
		sid := id(svc["ID"])
		name, _ := svc["Name"].(string)
		active, _ := svc["Active"].(bool)
		if sid > 0 && active {
			svcID = sid
			svcName = name
			break
		}
	}

	if barberID == 0 || svcID == 0 {
		t.Fatalf("setup falhou: barberID=%d svcID=%d — servidor rodando com seed?", barberID, svcID)
	}
	return
}

// nextAvailableSlot retorna a próxima data/hora disponível para o serviço dado.
func nextAvailableSlot(svcID uint) (date, slot string) {
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	for i := 1; i <= 14; i++ {
		d := time.Now().In(loc).AddDate(0, 0, i)
		if d.Weekday() == time.Sunday {
			continue
		}
		dateStr := d.Format("2006-01-02")
		var resp map[string]any
		pub("GET", fmt.Sprintf("/api/public/%s/availability?date=%s&product_id=%d",
			demoSlug, dateStr, svcID), nil, &resp)
		if slots, ok := resp["slots"].([]any); ok && len(slots) > 0 {
			first := slots[0].(map[string]any)
			return dateStr, first["start"].(string)
		}
	}
	return "", ""
}

// ============================================================
// WAVE 1 — VELOCIDADE DOS ENDPOINTS PRINCIPAIS
// ============================================================

func TestWaveSpeed(t *testing.T) {
	barberID, svcID, _ := waveSetup(t)
	date, _ := nextAvailableSlot(svcID)

	t.Log("\n══════════════════════════════════════════════════════════")
	t.Log("  WAVE 1 — VELOCIDADE DOS ENDPOINTS PRINCIPAIS")
	t.Log("══════════════════════════════════════════════════════════")

	slug := "/api/public/" + demoSlug

	var total time.Duration
	var count int
	add := func(d time.Duration) { total += d; count++ }

	add(measure(t, "GET /api/public/:slug/info", func() int {
		return pub("GET", slug+"/info", nil, nil)
	}))
	add(measure(t, "GET /api/public/:slug/services", func() int {
		return pub("GET", slug+"/services", nil, nil)
	}))
	add(measure(t, "GET /api/public/:slug/products", func() int {
		return pub("GET", slug+"/products", nil, nil)
	}))
	add(measure(t, "GET /api/public/:slug/availability", func() int {
		return pub("GET", fmt.Sprintf("%s/availability?date=%s&product_id=%d", slug, date, svcID), nil, nil)
	}))
	add(measure(t, "GET /api/me/barbershop", func() int {
		return get("/api/me/barbershop", nil)
	}))
	add(measure(t, "GET /api/me/appointments/date", func() int {
		return get("/api/me/appointments/date?date="+date, nil)
	}))
	add(measure(t, "GET /api/me/day-panel", func() int {
		return get("/api/me/day-panel?date="+date, nil)
	}))
	add(measure(t, "GET /api/me/dashboard", func() int {
		return get("/api/me/dashboard", nil)
	}))
	add(measure(t, "GET /api/me/financial", func() int {
		return get("/api/me/financial", nil)
	}))
	add(measure(t, "GET /api/me/clients", func() int {
		return get("/api/me/clients", nil)
	}))
	add(measure(t, "GET /api/me/payments", func() int {
		return get("/api/me/payments", nil)
	}))

	avg := total / time.Duration(count)
	t.Logf("\n  Média: %dms  |  %d endpoints  |  Total: %dms  |  barberID=%d svcID=%d",
		avg.Milliseconds(), count, total.Milliseconds(), barberID, svcID)
}

// ============================================================
// WAVE 2 — CACHE (2ª chamada deve ser mais rápida)
// ============================================================

func TestWaveCache(t *testing.T) {
	_, svcID, _ := waveSetup(t)
	date, _ := nextAvailableSlot(svcID)
	slug := "/api/public/" + demoSlug

	t.Log("\n══════════════════════════════════════════════════════════")
	t.Log("  WAVE 2 — CACHE (2ª chamada deve ser mais rápida)")
	t.Log("══════════════════════════════════════════════════════════")

	check := func(t *testing.T, label, path string) {
		t.Helper()
		r1 := measure(t, "1ª "+label, func() int { return pub("GET", path, nil, nil) })
		r2 := measure(t, "2ª "+label+" (cache)", func() int { return pub("GET", path, nil, nil) })
		if r2 < r1 {
			t.Logf("  ✓ cache: %.0fms → %.0fms (%.0f%% mais rápido)",
				float64(r1.Milliseconds()), float64(r2.Milliseconds()),
				float64(r1-r2)/float64(r1)*100)
		} else {
			t.Logf("  ℹ diferença imperceptível (ambas < 5ms ou latência de rede variável)")
		}
	}

	t.Run("slug_cache", func(t *testing.T) { check(t, "GET /info", slug+"/info") })
	t.Run("services_cache", func(t *testing.T) { check(t, "GET /services", slug+"/services") })
	t.Run("products_cache", func(t *testing.T) { check(t, "GET /products", slug+"/products") })
	t.Run("availability_cache", func(t *testing.T) {
		url := fmt.Sprintf("%s/availability?date=%s&product_id=%d", slug, date, svcID)
		check(t, "GET /availability", url)
	})
}

// ============================================================
// WAVE 3 — FLUXO COMPLETO DE AGENDAMENTO PAGO
// ============================================================

func TestWaveFluxoPago(t *testing.T) {
	barberID, svcID, svcName := waveSetup(t)

	t.Log("\n══════════════════════════════════════════════════════════")
	t.Log("  WAVE 3 — FLUXO COMPLETO DE AGENDAMENTO PAGO")
	t.Log("══════════════════════════════════════════════════════════")

	date, slot := nextAvailableSlot(svcID)
	if date == "" {
		t.Fatal("nenhum slot disponível nos próximos 14 dias")
	}
	t.Logf("  Serviço: %s (id=%d)  |  Barber: %d  |  Slot: %s %s", svcName, svcID, barberID, date, slot)

	// ── PASSO 1: Criar agendamento ──────────────────────────────
	var appt map[string]any
	d1 := measure(t, "POST /api/me/appointments (criar)", func() int {
		return post("/api/me/appointments", map[string]any{
			"client_name":  "Teste Wave Pago",
			"client_phone": "11900000099",
			"product_id":   svcID,
			"date":         date,
			"time":         slot,
			"notes":        "wave test — fluxo pago",
		}, &appt)
	})
	if appt["ID"] == nil {
		t.Fatalf("agendamento não criado: %v (%dms)", appt, d1.Milliseconds())
	}
	apptID := id(appt["ID"])
	apptStatus := appt["Status"]
	t.Logf("  → id=%d  status=%v  (%dms)", apptID, apptStatus, d1.Milliseconds())

	// ── PASSO 2: Se awaiting_payment → gerar payment e confirmar ─
	if apptStatus == "awaiting_payment" {
		t.Log("  → status awaiting_payment: criando MP preference...")

		var mpResp map[string]any
		d2 := measure(t, "POST /api/public/:slug/appointments/:id/payment/mp", func() int {
			return pub("POST",
				fmt.Sprintf("/api/public/%s/appointments/%d/payment/mp", demoSlug, apptID),
				map[string]any{"payer_email": "wave@test.com"},
				&mpResp)
		})
		t.Logf("  → preference: %v (%dms)", mpResp["preference_id"], d2.Milliseconds())

		paymentID := id(mpResp["payment_id"])
		if paymentID > 0 {
			var confirmResp map[string]any
			d3 := measure(t, "POST /api/dev/payments/:id/confirm (bypass mock)", func() int {
				return doJSON("POST",
					fmt.Sprintf("/api/dev/payments/%d/confirm", paymentID),
					nil, "", &confirmResp)
			})
			if confirmResp["ok"] == true {
				t.Logf("  ✓ pagamento %d confirmado via mock  (%dms)", paymentID, d3.Milliseconds())
			} else {
				t.Logf("  ⚠ bypass retornou: %v  (%dms)", confirmResp, d3.Milliseconds())
			}
		}
	} else {
		t.Logf("  → status %v: sem cobrança (política optional/none)", apptStatus)
	}

	// ── PASSO 3: Verificar agendamento no day-panel ──────────────
	var panel map[string]any
	d3 := measure(t, "GET /api/me/day-panel (verificar agendamento)", func() int {
		return get("/api/me/day-panel?date="+date, &panel)
	})
	if cards, ok := panel["cards"].([]any); ok {
		t.Logf("  → %d cards no painel para %s  (%dms)", len(cards), date, d3.Milliseconds())
	}

	// ── PASSO 4: Completar agendamento ──────────────────────────
	var closure map[string]any
	d4 := measure(t, "PUT /api/me/appointments/:id/complete", func() int {
		return put(fmt.Sprintf("/api/me/appointments/%d/complete", apptID), map[string]any{
			"payment_method":          "cash",
			"confirm_normal_charging": true,
			"operational_note":        "wave test — concluído",
		}, &closure)
	})
	if ap, ok := closure["appointment"].(map[string]any); ok {
		t.Logf("  ✓ status após complete: %v  (%dms)", ap["Status"], d4.Milliseconds())
	} else {
		t.Logf("  ⚠ complete retornou: %v  (%dms)", closure, d4.Milliseconds())
	}

	// ── PASSO 5: Verificar impacto nas métricas ──────────────────
	measure(t, "GET /api/me/summary (pós-conclusão)", func() int {
		return get("/api/me/summary", nil)
	})
	measure(t, "GET /api/me/payments/summary", func() int {
		return get(fmt.Sprintf("/api/me/payments/summary?from=%s&to=%s", date, date), nil)
	})
}

// ============================================================
// WAVE 4 — CONCORRÊNCIA (3 agendamentos simultâneos)
// ============================================================

func TestWaveConcorrencia(t *testing.T) {
	_, svcID, _ := waveSetup(t)

	t.Log("\n══════════════════════════════════════════════════════════")
	t.Log("  WAVE 4 — CONCORRÊNCIA (3 agendamentos em 3 slots distintos)")
	t.Log("══════════════════════════════════════════════════════════")

	// Busca 3 slots disponíveis em dias diferentes para evitar conflito real
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	var slotPairs [][2]string
	for i := 1; i <= 21 && len(slotPairs) < 3; i++ {
		d := time.Now().In(loc).AddDate(0, 0, i)
		if d.Weekday() == time.Sunday {
			continue
		}
		dateStr := d.Format("2006-01-02")
		var resp map[string]any
		pub("GET", fmt.Sprintf("/api/public/%s/availability?date=%s&product_id=%d",
			demoSlug, dateStr, svcID), nil, &resp)
		if slots, ok := resp["slots"].([]any); ok && len(slots) > 0 {
			first := slots[0].(map[string]any)
			slotPairs = append(slotPairs, [2]string{dateStr, first["start"].(string)})
		}
	}

	if len(slotPairs) < 3 {
		t.Skipf("menos de 3 slots disponíveis para teste de concorrência (%d encontrados)", len(slotPairs))
	}

	type result struct {
		date    string
		slot    string
		code    int
		elapsed time.Duration
		apptID  uint
	}

	results := make([]result, 3)
	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int, pair [2]string) {
			defer wg.Done()
			t0 := time.Now()
			var resp map[string]any
			code := post("/api/me/appointments", map[string]any{
				"client_name":  fmt.Sprintf("Concorrente %d", idx+1),
				"client_phone": fmt.Sprintf("1190000%04d", idx+50),
				"product_id":   svcID,
				"date":         pair[0],
				"time":         pair[1],
			}, &resp)
			r := result{date: pair[0], slot: pair[1], code: code, elapsed: time.Since(t0)}
			if resp["ID"] != nil {
				r.apptID = id(resp["ID"])
			}
			results[idx] = r
		}(i, slotPairs[i])
	}
	wg.Wait()
	totalTime := time.Since(start)

	created := 0
	for _, r := range results {
		status := "✓"
		if r.code != 201 {
			status = fmt.Sprintf("✗ HTTP %d", r.code)
		} else {
			created++
		}
		t.Logf("  %s %s %s  →  id=%-6d  %dms", status, r.date, r.slot, r.apptID, r.elapsed.Milliseconds())
	}
	t.Logf("\n  %d/3 criados em paralelo  |  tempo total: %dms  (sequencial seria ~%dms)",
		created, totalTime.Milliseconds(), results[0].elapsed.Milliseconds()+results[1].elapsed.Milliseconds()+results[2].elapsed.Milliseconds())

	if created == 3 {
		t.Log("  ✓ sem corrida detectada — 3 slots distintos criados corretamente")
	}
}

// ============================================================
// WAVE 5 — REGRAS DE NEGÓCIO E EDGE CASES
// ============================================================

func TestWaveRegras(t *testing.T) {
	_, svcID, _ := waveSetup(t)
	date, slot := nextAvailableSlot(svcID)

	t.Log("\n══════════════════════════════════════════════════════════")
	t.Log("  WAVE 5 — REGRAS DE NEGÓCIO E EDGE CASES")
	t.Log("══════════════════════════════════════════════════════════")

	t.Run("horario_passado_rejeitado", func(t *testing.T) {
		code := post("/api/me/appointments", map[string]any{
			"client_name":  "Passado",
			"client_phone": "11900000020",
			"product_id":   svcID,
			"date":         "2020-01-01",
			"time":         "10:00",
		}, nil)
		if code == 400 {
			t.Logf("  ✓ horário passado rejeitado → 400")
		} else {
			t.Errorf("  ✗ esperado 400, recebeu %d", code)
		}
	})

	t.Run("servico_inexistente_rejeitado", func(t *testing.T) {
		code := post("/api/me/appointments", map[string]any{
			"client_name":  "Servico Inv",
			"client_phone": "11900000021",
			"product_id":   uint(999999),
			"date":         date,
			"time":         slot,
		}, nil)
		if code == 400 {
			t.Logf("  ✓ serviço inexistente rejeitado → 400")
		} else {
			t.Errorf("  ✗ esperado 400, recebeu %d", code)
		}
	})

	t.Run("duplo_booking_mesmo_slot", func(t *testing.T) {
		// Usa slot diferente para não conflitar com outros testes
		loc, _ := time.LoadLocation("America/Sao_Paulo")
		d := time.Now().In(loc).AddDate(0, 0, 10)
		for d.Weekday() == time.Sunday {
			d = d.AddDate(0, 0, 1)
		}
		conflictDate := d.Format("2006-01-02")
		conflictSlot := "17:00"

		var r1 map[string]any
		code1 := post("/api/me/appointments", map[string]any{
			"client_name":  "Duplo A",
			"client_phone": "11900000030",
			"product_id":   svcID,
			"date":         conflictDate,
			"time":         conflictSlot,
		}, &r1)
		code2 := post("/api/me/appointments", map[string]any{
			"client_name":  "Duplo B",
			"client_phone": "11900000031",
			"product_id":   svcID,
			"date":         conflictDate,
			"time":         conflictSlot,
		}, nil)

		if code1 == 201 && code2 == 400 {
			t.Logf("  ✓ conflito de horário detectado: 1º=201, 2º=400")
		} else if code1 == 400 {
			t.Logf("  ℹ 1º também rejeitado (%d) — slot pode estar ocupado por outro teste", code1)
		} else {
			t.Logf("  ℹ resultado: 1º=%d 2º=%d", code1, code2)
		}
	})

	t.Run("completar_depois_cancelar_invalido", func(t *testing.T) {
		loc, _ := time.LoadLocation("America/Sao_Paulo")
		d := time.Now().In(loc).AddDate(0, 0, 11)
		for d.Weekday() == time.Sunday {
			d = d.AddDate(0, 0, 1)
		}

		var appt map[string]any
		post("/api/me/appointments", map[string]any{
			"client_name":  "Complete Cancel",
			"client_phone": "11900000032",
			"product_id":   svcID,
			"date":         d.Format("2006-01-02"),
			"time":         "16:30",
		}, &appt)
		if appt["ID"] == nil {
			t.Skip("não criou agendamento base")
		}
		apptID := id(appt["ID"])

		put(fmt.Sprintf("/api/me/appointments/%d/complete", apptID), map[string]any{
			"payment_method": "cash", "confirm_normal_charging": true,
		}, nil)

		code := put(fmt.Sprintf("/api/me/appointments/%d/cancel", apptID), nil, nil)
		if code == 400 {
			t.Logf("  ✓ cancelar completado rejeitado → 400")
		} else {
			t.Errorf("  ✗ esperado 400, recebeu %d", code)
		}
	})

	t.Run("no_show_depois_completar_invalido", func(t *testing.T) {
		loc, _ := time.LoadLocation("America/Sao_Paulo")
		d := time.Now().In(loc).AddDate(0, 0, 12)
		for d.Weekday() == time.Sunday {
			d = d.AddDate(0, 0, 1)
		}

		var appt map[string]any
		post("/api/me/appointments", map[string]any{
			"client_name":  "NoShow Inv",
			"client_phone": "11900000033",
			"product_id":   svcID,
			"date":         d.Format("2006-01-02"),
			"time":         "16:00",
		}, &appt)
		if appt["ID"] == nil {
			t.Skip("não criou agendamento base")
		}
		apptID := id(appt["ID"])

		put(fmt.Sprintf("/api/me/appointments/%d/complete", apptID), map[string]any{
			"payment_method": "cash", "confirm_normal_charging": true,
		}, nil)

		code := put(fmt.Sprintf("/api/me/appointments/%d/no-show", apptID), nil, nil)
		if code == 400 {
			t.Logf("  ✓ no-show em completado rejeitado → 400")
		} else {
			t.Errorf("  ✗ esperado 400, recebeu %d", code)
		}
	})

	t.Run("token_invalido_bloqueado", func(t *testing.T) {
		code := doJSON("GET", "/api/me/dashboard", nil, "token_invalido_xpto", nil)
		if code == 401 {
			t.Logf("  ✓ token inválido → 401")
		} else {
			t.Errorf("  ✗ esperado 401, recebeu %d", code)
		}
	})

	t.Run("sem_token_bloqueado", func(t *testing.T) {
		code := doJSON("GET", "/api/me/clients", nil, "", nil)
		if code == 401 {
			t.Logf("  ✓ sem token → 401")
		} else {
			t.Errorf("  ✗ esperado 401, recebeu %d", code)
		}
	})

	t.Run("rota_publica_sem_token_acessivel", func(t *testing.T) {
		code := doJSON("GET", "/api/public/"+demoSlug+"/info", nil, "", nil)
		if code == 200 {
			t.Logf("  ✓ rota pública acessível sem token → 200")
		} else {
			t.Errorf("  ✗ esperado 200, recebeu %d", code)
		}
	})
}

// ============================================================
// WAVE 6 — RELATÓRIO FINAL DE PERFORMANCE
// ============================================================

func TestWaveRelatorio(t *testing.T) {
	_, svcID, _ := waveSetup(t)
	date, _ := nextAvailableSlot(svcID)

	t.Log("\n══════════════════════════════════════════════════════════")
	t.Log("  WAVE 6 — RELATÓRIO FINAL DE PERFORMANCE")
	t.Log("══════════════════════════════════════════════════════════")

	slug := "/api/public/" + demoSlug

	endpoints := []struct {
		label  string
		method string
		path   string
		auth   bool
	}{
		{"Health check", "GET", "/health", false},
		{"Login", "POST", "/api/auth/login", false},
		{"Barbearia info (público)", "GET", slug + "/info", false},
		{"Serviços (público)", "GET", slug + "/services", false},
		{"Serviços (público, 2ª — cache)", "GET", slug + "/services", false},
		{"Produtos (público)", "GET", slug + "/products", false},
		{"Disponibilidade (público)", "GET", fmt.Sprintf("%s/availability?date=%s&product_id=%d", slug, date, svcID), false},
		{"Disponibilidade (2ª — cache horários)", "GET", fmt.Sprintf("%s/availability?date=%s&product_id=%d", slug, date, svcID), false},
		{"Me (privado)", "GET", "/api/me", true},
		{"Agendamentos do dia", "GET", "/api/me/appointments/date?date=" + date, true},
		{"Day panel", "GET", "/api/me/day-panel?date=" + date, true},
		{"Dashboard", "GET", "/api/me/dashboard", true},
		{"Financeiro", "GET", "/api/me/financial", true},
		{"Clientes", "GET", "/api/me/clients", true},
		{"Pagamentos", "GET", "/api/me/payments", true},
		{"Planos", "GET", "/api/me/plans", true},
		{"Assinaturas", "GET", "/api/me/subscriptions", true},
		{"Resumo operacional", "GET", "/api/me/summary", true},
		{"Audit logs", "GET", "/api/me/audit-logs", true},
	}

	type bench struct {
		label   string
		elapsed time.Duration
		code    int
	}

	results := make([]bench, 0, len(endpoints))
	for _, ep := range endpoints {
		var body any
		if ep.method == "POST" && ep.path == "/api/auth/login" {
			body = map[string]any{"email": demoEmail, "password": demoPassword}
		}
		tok := ""
		if ep.auth {
			tok = bearer
		}
		start := time.Now()
		code := doJSON(ep.method, ep.path, body, tok, nil)
		results = append(results, bench{ep.label, time.Since(start), code})
	}

	t.Log("\n  Endpoint                                    ms     Status")
	t.Log("  ───────────────────────────────────────────────────────────")

	var totalMs int64
	slow := 0
	for _, r := range results {
		status := "OK"
		if r.code >= 500 {
			status = fmt.Sprintf("ERR %d", r.code)
		} else if r.code >= 400 {
			status = fmt.Sprintf("WARN %d", r.code)
		} else if r.code == 0 {
			status = "TIMEOUT"
		}
		ms := r.elapsed.Milliseconds()
		totalMs += ms
		marker := " "
		if ms > 500 {
			marker = "⚠"
			slow++
		} else if ms > 200 {
			marker = "·"
		}
		t.Logf(" %s %-42s %4dms  %s", marker, r.label, ms, status)
	}

	avg := totalMs / int64(len(results))
	t.Logf("\n  Média: %dms  |  Total: %dms  |  Endpoints: %d  |  Lentos (>500ms): %d",
		avg, totalMs, len(results), slow)

	switch {
	case avg > 500:
		t.Logf("  ⚠ média alta — verifique queries sem índice ou cold start do Neon")
	case avg > 200:
		t.Logf("  · aceitável para Neon serverless (cold start de ~200ms incluso)")
	default:
		t.Logf("  ✓ performance excelente")
	}
}
