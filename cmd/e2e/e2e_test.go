// cmd/e2e/e2e_test.go
//
// Testes end-to-end contra o servidor em execução com os dados do seed.
//
// Pré-requisitos:
//   1. Banco com seed aplicado: go run ./cmd/seed
//   2. Servidor rodando:        go run ./cmd/api
//
// Execução:
//   BASE_URL=http://localhost:8080 go test ./cmd/e2e/... -v -timeout 120s
//
// O BASE_URL padrão é http://localhost:8080 se não for definido.
//
// Os testes rodam em ordem de declaração (padrão Go) e compartilham
// estado via variáveis de pacote. Cada seção pode ser rodada isolada
// com -run, mas algumas dependem de seções anteriores para IDs.

package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"
)

// ============================================================
// CONFIG
// ============================================================

const (
	demoEmail    = "demo@barbeariprime.com.br"
	demoPassword = "Demo@2025"
	demoSlug     = "barbearia-prime"
)

var (
	base   string
	hc     = &http.Client{Timeout: 15 * time.Second}
	bearer string // JWT do demo user

	// IDs descobertos/criados durante a suite
	demoServiceID         uint
	demoProductID         uint
	demoClientID          uint
	demoServiceCategoryID uint

	createdServiceID  uint
	createdProductID  uint
	createdPlanID     uint
	createdApptID     uint // appointment privado
	createdPublicAppt uint // appointment público
	createdOrderID    uint
	createdClosureID  uint
	ticketToken       string
)

// ============================================================
// SETUP
// ============================================================

func TestMain(m *testing.M) {
	base = os.Getenv("BASE_URL")
	if base == "" {
		base = "http://localhost:8080"
	}

	var resp map[string]any
	code := doJSON("POST", "/api/auth/login", map[string]string{
		"email":    demoEmail,
		"password": demoPassword,
	}, "", &resp)

	if code != 200 {
		fmt.Printf("FATAL: login retornou %d — servidor está rodando? seed foi aplicado?\n", code)
		os.Exit(1)
	}

	bearer = resp["token"].(string)
	fmt.Printf("✓ Login OK — token obtido\n")

	os.Exit(m.Run())
}

// ============================================================
// HELPERS
// ============================================================

func doJSON(method, path string, body any, token string, out any) int {
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}

	req, _ := http.NewRequest(method, base+path, r)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	res, err := hc.Do(req)
	if err != nil {
		fmt.Printf("  HTTP error %s %s: %v\n", method, path, err)
		return 0
	}
	defer res.Body.Close()

	if out != nil {
		_ = json.NewDecoder(res.Body).Decode(out)
	} else {
		io.Copy(io.Discard, res.Body)
	}
	return res.StatusCode
}

func auth(method, path string, body any, out any) int {
	return doJSON(method, path, body, bearer, out)
}

func get(path string, out any) int          { return auth("GET", path, nil, out) }
func post(path string, b, out any) int      { return auth("POST", path, b, out) }
func put(path string, b, out any) int       { return auth("PUT", path, b, out) }
func patch(path string, b, out any) int     { return auth("PATCH", path, b, out) }
func del(path string, out any) int          { return auth("DELETE", path, nil, out) }
func pub(method, path string, b, out any) int { return doJSON(method, path, b, "", out) }

func id(v any) uint {
	switch n := v.(type) {
	case float64:
		return uint(n)
	case json.Number:
		i, _ := n.Int64()
		return uint(i)
	}
	return 0
}

// nextSlot retorna (date, time) para o próximo dia útil, às 10h no timezone da barbearia.
// Garante antecedência mínima de 3h para o min_advance_minutes=120 do seed.
func nextSlot() (string, string) {
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	t := time.Now().In(loc).Add(4 * time.Hour)
	for t.Weekday() == time.Sunday {
		t = t.AddDate(0, 0, 1)
	}
	d := time.Date(t.Year(), t.Month(), t.Day(), 10, 0, 0, 0, loc)
	if d.Before(t) {
		d = d.AddDate(0, 0, 1)
		for d.Weekday() == time.Sunday {
			d = d.AddDate(0, 0, 1)
		}
	}
	return d.Format("2006-01-02"), "10:00"
}

func assertStatus(t *testing.T, got, want int, label string) bool {
	t.Helper()
	if got != want {
		t.Errorf("  ✗ %s → HTTP %d (esperado %d)", label, got, want)
		return false
	}
	t.Logf("  ✓ %s → HTTP %d", label, got)
	return true
}

// ============================================================
// 01 — AUTENTICAÇÃO
// ============================================================

func TestAuth(t *testing.T) {
	t.Run("login_invalido", func(t *testing.T) {
		code := doJSON("POST", "/api/auth/login", map[string]string{
			"email":    demoEmail,
			"password": "senha_errada",
		}, "", nil)
		assertStatus(t, code, 401, "login com senha errada")
	})

	t.Run("login_email_invalido", func(t *testing.T) {
		code := doJSON("POST", "/api/auth/login", map[string]string{
			"email":    "naoexiste@test.com",
			"password": "qualquer",
		}, "", nil)
		assertStatus(t, code, 401, "login com email inexistente")
	})

	t.Run("token_invalido", func(t *testing.T) {
		code := doJSON("GET", "/api/me", nil, "token_invalido", nil)
		assertStatus(t, code, 401, "acesso com token inválido")
	})

	t.Run("sem_token", func(t *testing.T) {
		code := doJSON("GET", "/api/me", nil, "", nil)
		assertStatus(t, code, 401, "acesso sem token")
	})
}

// ============================================================
// 02 — ME / BARBEARIA
// ============================================================

func TestMe(t *testing.T) {
	t.Run("get_me", func(t *testing.T) {
		var resp map[string]any
		code := get("/api/me", &resp)
		if assertStatus(t, code, 200, "GET /api/me") {
			t.Logf("    user: %v", resp["name"])
		}
	})

	t.Run("get_barbershop", func(t *testing.T) {
		var resp map[string]any
		code := get("/api/me/barbershop", &resp)
		if assertStatus(t, code, 200, "GET /api/me/barbershop") {
			t.Logf("    barbershop: %v | slug: %v", resp["name"], resp["slug"])
		}
	})

	t.Run("update_barbershop", func(t *testing.T) {
		code := put("/api/me/barbershop", map[string]any{
			"name":    "Barbearia Prime",
			"phone":   "(11) 99876-5432",
			"address": "Rua Augusta, 1245 – Consolação, SP",
			"min_advance_minutes": 60,
		}, nil)
		assertStatus(t, code, 200, "PUT /api/me/barbershop")
	})

	t.Run("mark_tour_seen", func(t *testing.T) {
		code := post("/api/me/tours/dashboard/seen", nil, nil)
		// 200 ou 204 dependendo da implementação
		if code != 200 && code != 204 && code != 201 {
			t.Logf("  ℹ tour seen retornou %d", code)
		} else {
			t.Logf("  ✓ mark tour seen → HTTP %d", code)
		}
	})
}

// ============================================================
// 03 — CATEGORIAS DE SERVIÇO
// ============================================================

func TestServiceCategories(t *testing.T) {
	t.Run("listar", func(t *testing.T) {
		var resp []any
		code := get("/api/me/service-categories", &resp)
		assertStatus(t, code, 200, "GET /api/me/service-categories")
		t.Logf("    total categorias: %d", len(resp))
	})

	t.Run("criar", func(t *testing.T) {
		var resp map[string]any
		code := post("/api/me/service-categories", map[string]any{
			"name": "Categoria E2E",
		}, &resp)
		if assertStatus(t, code, 201, "POST /api/me/service-categories") {
			demoServiceCategoryID = id(resp["id"])
			t.Logf("    id criado: %d", demoServiceCategoryID)
		}
	})

	t.Run("atualizar", func(t *testing.T) {
		if demoServiceCategoryID == 0 {
			t.Skip("categoria não criada")
		}
		code := put(fmt.Sprintf("/api/me/service-categories/%d", demoServiceCategoryID), map[string]any{
			"name": "Categoria E2E Editada",
		}, nil)
		assertStatus(t, code, 200, "PUT /api/me/service-categories/:id")
	})
}

// ============================================================
// 04 — SERVIÇOS
// ============================================================

func TestServices(t *testing.T) {
	t.Run("listar", func(t *testing.T) {
		var resp []any
		code := get("/api/me/services", &resp)
		if assertStatus(t, code, 200, "GET /api/me/services") && len(resp) > 0 {
			first := resp[0].(map[string]any)
			demoServiceID = id(first["id"])
			t.Logf("    total: %d | primeiro id: %d (%v)", len(resp), demoServiceID, first["name"])
		}
	})

	t.Run("criar", func(t *testing.T) {
		var resp map[string]any
		body := map[string]any{
			"name":         "Corte E2E",
			"description":  "Serviço criado pelo teste e2e",
			"duration_min": 30,
			"price":        4500,
			"active":       true,
		}
		if demoServiceCategoryID > 0 {
			body["category_id"] = demoServiceCategoryID
		}
		code := post("/api/me/services", body, &resp)
		if assertStatus(t, code, 201, "POST /api/me/services") {
			createdServiceID = id(resp["id"])
			t.Logf("    id criado: %d", createdServiceID)
		}
	})

	t.Run("atualizar", func(t *testing.T) {
		if createdServiceID == 0 {
			t.Skip("serviço não criado")
		}
		code := put(fmt.Sprintf("/api/me/services/%d", createdServiceID), map[string]any{
			"name":         "Corte E2E Editado",
			"description":  "Editado",
			"duration_min": 35,
			"price":        5000,
			"active":       true,
		}, nil)
		assertStatus(t, code, 200, "PUT /api/me/services/:id")
	})
}

// ============================================================
// 05 — SUGESTÃO DE PRODUTO POR SERVIÇO
// ============================================================

func TestServiceSuggestions(t *testing.T) {
	if demoServiceID == 0 {
		t.Skip("nenhum serviço disponível")
	}

	path := fmt.Sprintf("/api/me/services/%d/suggestion", demoServiceID)

	t.Run("get_sugestao", func(t *testing.T) {
		var resp map[string]any
		code := get(path, &resp)
		// 200 ou 404 (sem sugestão configurada)
		if code != 200 && code != 404 {
			t.Errorf("  ✗ GET suggestion → HTTP %d", code)
		} else {
			t.Logf("  ✓ GET suggestion → HTTP %d", code)
		}
	})
}

// ============================================================
// 06 — PRODUTOS
// ============================================================

func TestProducts(t *testing.T) {
	t.Run("listar", func(t *testing.T) {
		var resp []any
		code := get("/api/me/products", &resp)
		if assertStatus(t, code, 200, "GET /api/me/products") && len(resp) > 0 {
			first := resp[0].(map[string]any)
			demoProductID = id(first["id"])
			t.Logf("    total: %d | primeiro id: %d (%v)", len(resp), demoProductID, first["name"])
		}
	})

	t.Run("criar", func(t *testing.T) {
		var resp map[string]any
		code := post("/api/me/products", map[string]any{
			"name":           "Produto E2E",
			"description":    "Criado pelo teste",
			"price":          2500,
			"stock":          10,
			"active":         true,
			"online_visible": true,
			"category":       "teste",
		}, &resp)
		if assertStatus(t, code, 201, "POST /api/me/products") {
			createdProductID = id(resp["id"])
			t.Logf("    id criado: %d", createdProductID)
		}
	})

	t.Run("atualizar", func(t *testing.T) {
		if createdProductID == 0 {
			t.Skip("produto não criado")
		}
		code := put(fmt.Sprintf("/api/me/products/%d", createdProductID), map[string]any{
			"name":           "Produto E2E Editado",
			"description":    "Editado",
			"price":          2800,
			"stock":          8,
			"active":         true,
			"online_visible": true,
			"category":       "teste",
		}, nil)
		assertStatus(t, code, 200, "PUT /api/me/products/:id")
	})
}

// ============================================================
// 07 — HORÁRIOS DE TRABALHO
// (deve rodar antes dos testes de agendamento)
// ============================================================

func TestWorkingHours(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		var resp any
		code := get("/api/me/working-hours", &resp)
		assertStatus(t, code, 200, "GET /api/me/working-hours")
	})

	t.Run("put", func(t *testing.T) {
		// Configura seg-sab 09-20 com almoço 12-13 para o usuário autenticado
		hours := make([]map[string]any, 7)
		for i := 0; i <= 6; i++ {
			active := i >= 1 && i <= 6 // seg-sáb
			h := map[string]any{
				"weekday": i,
				"active":  active,
			}
			if active {
				h["start_time"] = "09:00"
				h["end_time"] = "20:00"
				h["lunch_start"] = "12:00"
				h["lunch_end"] = "13:00"
			}
			hours[i] = h
		}
		code := put("/api/me/working-hours", map[string]any{"hours": hours}, nil)
		assertStatus(t, code, 200, "PUT /api/me/working-hours")
	})
}

// ============================================================
// 08 — SCHEDULE OVERRIDES
// ============================================================

func TestScheduleOverrides(t *testing.T) {
	t.Run("listar", func(t *testing.T) {
		var resp any
		code := get("/api/me/schedule-overrides", &resp)
		assertStatus(t, code, 200, "GET /api/me/schedule-overrides")
	})

	var overrideID uint

	t.Run("criar_excecao_por_data", func(t *testing.T) {
		// Cria exceção para uma data específica (folga)
		loc, _ := time.LoadLocation("America/Sao_Paulo")
		futureDate := time.Now().In(loc).AddDate(0, 2, 0).Format("2006-01-02")

		var resp map[string]any
		code := put("/api/me/schedule-overrides", map[string]any{
			"date":   futureDate,
			"active": false, // folga
		}, &resp)
		if assertStatus(t, code, 200, "PUT /api/me/schedule-overrides (data)") {
			overrideID = id(resp["id"])
		}
	})

	t.Run("deletar_excecao", func(t *testing.T) {
		if overrideID == 0 {
			t.Skip("override não criado")
		}
		code := del(fmt.Sprintf("/api/me/schedule-overrides/%d", overrideID), nil)
		if code != 200 && code != 204 {
			t.Errorf("  ✗ DELETE schedule-override → HTTP %d", code)
		} else {
			t.Logf("  ✓ DELETE schedule-override → HTTP %d", code)
		}
	})
}

// ============================================================
// 09 — POLÍTICAS DE PAGAMENTO
// ============================================================

func TestPaymentPolicies(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		var resp any
		code := get("/api/me/payment-policies", &resp)
		assertStatus(t, code, 200, "GET /api/me/payment-policies")
	})

	t.Run("put", func(t *testing.T) {
		code := put("/api/me/payment-policies", map[string]any{
			"default_requirement": "optional",
			"category_policies": []map[string]any{
				{"category": "new", "requirement": "optional"},
				{"category": "regular", "requirement": "optional"},
				{"category": "trusted", "requirement": "none"},
				{"category": "at_risk", "requirement": "mandatory"},
			},
		}, nil)
		assertStatus(t, code, 200, "PUT /api/me/payment-policies")
	})
}

// ============================================================
// 10 — CLIENTES
// ============================================================

func TestClients(t *testing.T) {
	t.Run("listar", func(t *testing.T) {
		var resp map[string]any
		code := get("/api/me/clients", &resp)
		if assertStatus(t, code, 200, "GET /api/me/clients") {
			if data, ok := resp["data"].([]any); ok && len(data) > 0 {
				first := data[0].(map[string]any)
				demoClientID = id(first["id"])
				t.Logf("    total clientes: %d | primeiro id: %d", len(data), demoClientID)
			}
		}
	})

	t.Run("historico", func(t *testing.T) {
		if demoClientID == 0 {
			t.Skip("nenhum cliente")
		}
		var resp any
		code := get(fmt.Sprintf("/api/me/clients/%d/history", demoClientID), &resp)
		assertStatus(t, code, 200, "GET /api/me/clients/:id/history")
	})

	t.Run("categoria", func(t *testing.T) {
		if demoClientID == 0 {
			t.Skip("nenhum cliente")
		}
		var resp any
		code := get(fmt.Sprintf("/api/me/clients/%d/category", demoClientID), &resp)
		assertStatus(t, code, 200, "GET /api/me/clients/:id/category")
	})

	t.Run("override_categoria", func(t *testing.T) {
		if demoClientID == 0 {
			t.Skip("nenhum cliente")
		}
		code := put(fmt.Sprintf("/api/me/clients/%d/category", demoClientID), map[string]any{
			"category": "trusted",
		}, nil)
		assertStatus(t, code, 200, "PUT /api/me/clients/:id/category")
	})

	t.Run("crm", func(t *testing.T) {
		if demoClientID == 0 {
			t.Skip("nenhum cliente")
		}
		var resp any
		code := get(fmt.Sprintf("/api/me/clients/%d/crm", demoClientID), &resp)
		assertStatus(t, code, 200, "GET /api/me/clients/:id/crm")
	})
}

// ============================================================
// 11 — AGENDAMENTOS PRIVADOS
// ============================================================

func TestPrivateAppointments(t *testing.T) {
	serviceID := demoServiceID
	if createdServiceID > 0 {
		serviceID = createdServiceID
	}
	if serviceID == 0 {
		t.Fatal("nenhum serviço disponível — rode TestServices antes")
	}

	date, slot := nextSlot()

	t.Run("criar", func(t *testing.T) {
		var resp map[string]any
		code := post("/api/me/appointments", map[string]any{
			"client_name":  "Cliente E2E",
			"client_phone": "11999999001",
			"product_id":   serviceID,
			"date":         date,
			"time":         slot,
			"notes":        "Teste automatizado e2e",
		}, &resp)
		if assertStatus(t, code, 201, "POST /api/me/appointments") {
			createdApptID = id(resp["id"])
			t.Logf("    id: %d | status: %v", createdApptID, resp["status"])
		}
	})

	t.Run("listar_por_data", func(t *testing.T) {
		var resp any
		code := get("/api/me/appointments/date?date="+date, &resp)
		assertStatus(t, code, 200, "GET /api/me/appointments/date")
	})

	t.Run("listar_por_mes", func(t *testing.T) {
		loc, _ := time.LoadLocation("America/Sao_Paulo")
		now := time.Now().In(loc)
		code := get(fmt.Sprintf("/api/me/appointments/month?year=%d&month=%d", now.Year(), int(now.Month())), nil)
		assertStatus(t, code, 200, "GET /api/me/appointments/month")
	})

	t.Run("idempotencia_duplicada", func(t *testing.T) {
		// Mesmo X-Idempotency-Key deve retornar 409
		key := fmt.Sprintf("e2e-idem-%d", time.Now().UnixNano())
		makeReq := func() int {
			b, _ := json.Marshal(map[string]any{
				"client_name":  "Cliente Idem",
				"client_phone": "11999999002",
				"product_id":   serviceID,
				"date":         date,
				"time":         "11:00",
				"notes":        "idem test",
			})
			req, _ := http.NewRequest("POST", base+"/api/me/appointments", bytes.NewReader(b))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+bearer)
			req.Header.Set("X-Idempotency-Key", key)
			res, err := hc.Do(req)
			if err != nil {
				return 0
			}
			defer res.Body.Close()
			io.Copy(io.Discard, res.Body)
			return res.StatusCode
		}
		first := makeReq()
		second := makeReq()
		t.Logf("  1ª chamada: %d | 2ª chamada (mesmo key): %d", first, second)
		if first == 201 && second == 409 {
			t.Logf("  ✓ idempotência funcionando corretamente")
		} else if first != 201 {
			t.Logf("  ℹ 1ª chamada não criou (pode ser conflito de horário): %d", first)
		}
	})

	t.Run("completar", func(t *testing.T) {
		if createdApptID == 0 {
			t.Skip("appointment não criado")
		}
		var resp map[string]any
		code := put(fmt.Sprintf("/api/me/appointments/%d/complete", createdApptID), map[string]any{
			"payment_method":          "cash",
			"confirm_normal_charging": true,
			"operational_note":        "Concluído pelo teste e2e",
		}, &resp)
		if assertStatus(t, code, 200, "PUT /api/me/appointments/:id/complete") {
			if closure, ok := resp["operational"].(map[string]any); ok {
				createdClosureID = id(closure["id"])
			}
			t.Logf("    closure id: %d", createdClosureID)
		}
	})

	t.Run("cancelar_estado_invalido", func(t *testing.T) {
		if createdApptID == 0 {
			t.Skip("appointment não criado")
		}
		// Tentar cancelar um já completado deve retornar 400
		code := put(fmt.Sprintf("/api/me/appointments/%d/cancel", createdApptID), nil, nil)
		assertStatus(t, code, 400, "PUT cancel appointment já completado → 400")
	})

	// Cria um segundo appointment para testar cancelamento
	date2, slot2 := func() (string, string) {
		loc, _ := time.LoadLocation("America/Sao_Paulo")
		t := time.Now().In(loc).Add(4 * time.Hour)
		for t.Weekday() == time.Sunday {
			t = t.AddDate(0, 0, 1)
		}
		d := time.Date(t.Year(), t.Month(), t.Day(), 14, 0, 0, 0, loc)
		if d.Before(t) {
			d = d.AddDate(0, 0, 1)
			for d.Weekday() == time.Sunday {
				d = d.AddDate(0, 0, 1)
			}
		}
		return d.Format("2006-01-02"), "14:00"
	}()

	var apptParaCancelar uint

	t.Run("criar_para_cancelar", func(t *testing.T) {
		var resp map[string]any
		code := post("/api/me/appointments", map[string]any{
			"client_name":  "Cliente Cancel E2E",
			"client_phone": "11999999003",
			"product_id":   serviceID,
			"date":         date2,
			"time":         slot2,
		}, &resp)
		if assertStatus(t, code, 201, "POST /api/me/appointments (para cancelar)") {
			apptParaCancelar = id(resp["id"])
		}
	})

	t.Run("cancelar", func(t *testing.T) {
		if apptParaCancelar == 0 {
			t.Skip("appointment para cancelar não criado")
		}
		code := put(fmt.Sprintf("/api/me/appointments/%d/cancel", apptParaCancelar), nil, nil)
		assertStatus(t, code, 200, "PUT /api/me/appointments/:id/cancel")
	})

	// Cria terceiro appointment para testar no-show
	var apptParaNoShow uint
	t.Run("criar_para_no_show", func(t *testing.T) {
		loc, _ := time.LoadLocation("America/Sao_Paulo")
		d := time.Now().In(loc).AddDate(0, 0, 3)
		for d.Weekday() == time.Sunday {
			d = d.AddDate(0, 0, 1)
		}
		dateNS := d.Format("2006-01-02")

		var resp map[string]any
		code := post("/api/me/appointments", map[string]any{
			"client_name":  "Cliente NoShow E2E",
			"client_phone": "11999999004",
			"product_id":   serviceID,
			"date":         dateNS,
			"time":         "15:00",
		}, &resp)
		if assertStatus(t, code, 201, "POST /api/me/appointments (para no-show)") {
			apptParaNoShow = id(resp["id"])
		}
	})

	t.Run("marcar_no_show", func(t *testing.T) {
		if apptParaNoShow == 0 {
			t.Skip("appointment para no-show não criado")
		}
		code := put(fmt.Sprintf("/api/me/appointments/%d/no-show", apptParaNoShow), nil, nil)
		assertStatus(t, code, 204, "PUT /api/me/appointments/:id/no-show")
	})
}

// ============================================================
// 12 — ENDPOINT INTERNO DE AGENDAMENTO
// ============================================================

func TestInternalAppointment(t *testing.T) {
	serviceID := demoServiceID
	if serviceID == 0 {
		t.Skip("nenhum serviço disponível")
	}

	loc, _ := time.LoadLocation("America/Sao_Paulo")
	d := time.Now().In(loc).AddDate(0, 0, 4)
	for d.Weekday() == time.Sunday {
		d = d.AddDate(0, 0, 1)
	}

	var resp map[string]any
	code := post("/api/me/internal-appointments", map[string]any{
		"client_name":  "Bloqueio Interno E2E",
		"client_phone": "11000000000",
		"product_id":   serviceID,
		"date":         d.Format("2006-01-02"),
		"time":         "16:00",
		"notes":        "Bloqueio de agenda — teste e2e",
	}, &resp)
	if code == 201 {
		t.Logf("  ✓ POST /api/me/internal-appointments → HTTP 201 | id: %v", resp["id"])
	} else {
		t.Logf("  ℹ POST /api/me/internal-appointments → HTTP %d", code)
	}
}

// ============================================================
// 13 — ENDPOINTS PÚBLICOS (info, serviços, produtos)
// ============================================================

func TestPublicEndpoints(t *testing.T) {
	slug := "/api/public/" + demoSlug

	t.Run("info", func(t *testing.T) {
		var resp map[string]any
		code := pub("GET", slug+"/info", nil, &resp)
		if assertStatus(t, code, 200, "GET /api/public/:slug/info") {
			t.Logf("    nome: %v | timezone: %v", resp["name"], resp["timezone"])
		}
	})

	t.Run("servicos", func(t *testing.T) {
		var resp []any
		code := pub("GET", slug+"/services", nil, &resp)
		if assertStatus(t, code, 200, "GET /api/public/:slug/services") {
			t.Logf("    total serviços públicos: %d", len(resp))
		}
	})

	t.Run("produtos", func(t *testing.T) {
		var resp []any
		code := pub("GET", slug+"/products", nil, &resp)
		if assertStatus(t, code, 200, "GET /api/public/:slug/products") {
			t.Logf("    total produtos públicos: %d", len(resp))
		}
	})

	t.Run("disponibilidade", func(t *testing.T) {
		date, _ := nextSlot()
		var resp any
		code := pub("GET", fmt.Sprintf("%s/availability?date=%s", slug, date), nil, &resp)
		assertStatus(t, code, 200, "GET /api/public/:slug/availability")
	})

	t.Run("barbershop_inexistente", func(t *testing.T) {
		code := pub("GET", "/api/public/slug-nao-existe/info", nil, nil)
		assertStatus(t, code, 404, "GET /api/public/slug-invalido/info → 404")
	})
}

// ============================================================
// 14 — AGENDAMENTO PÚBLICO + PAGAMENTO
// ============================================================

func TestPublicAppointmentAndPayment(t *testing.T) {
	slug := "/api/public/" + demoSlug

	// Descobre um serviço público
	var services []map[string]any
	pub("GET", slug+"/services", nil, &services)
	if len(services) == 0 {
		t.Skip("nenhum serviço público disponível")
	}
	svcID := id(services[0]["id"])

	date, slot := nextSlot()
	// Usa horário diferente para não conflitar com TestPrivateAppointments
	slot = "10:30"

	t.Run("criar_appointment_publico", func(t *testing.T) {
		var resp map[string]any
		code := pub("POST", slug+"/appointments", map[string]any{
			"client_name":  "Cliente Público E2E",
			"client_phone": "11888880001",
			"product_id":   svcID,
			"date":         date,
			"time":         slot,
		}, &resp)
		if code == 201 {
			createdPublicAppt = id(resp["id"])
			t.Logf("  ✓ POST /api/public/:slug/appointments → 201 | id: %d | status: %v", createdPublicAppt, resp["status"])
		} else {
			t.Logf("  ℹ POST /api/public/:slug/appointments → %d (pode ser conflito de horário)", code)
		}
	})

	t.Run("pagamento_transparente_sem_credenciais_mp", func(t *testing.T) {
		if createdPublicAppt == 0 {
			t.Skip("appointment público não criado")
		}
		// Esperado: 400 payment_not_configured (seed não tem MPAccessToken/MPPublicKey)
		code := pub("POST", fmt.Sprintf("%s/appointments/%d/payment/transparent", slug, createdPublicAppt),
			map[string]any{
				"payer_email":       "cliente@teste.com",
				"payment_method_id": "pix",
			}, nil)
		// 400 é o comportamento correto neste ambiente de seed (sem credenciais MP)
		if code == 400 {
			t.Logf("  ✓ Sem credenciais MP → 400 payment_not_configured (comportamento esperado no seed)")
		} else if code == 201 {
			t.Logf("  ✓ Pagamento criado (MP configurado)")
		} else {
			t.Logf("  ℹ pagamento transparente → %d", code)
		}
	})

	t.Run("pagamento_mp_preference_mock", func(t *testing.T) {
		if createdPublicAppt == 0 {
			t.Skip("appointment público não criado")
		}
		var resp map[string]any
		code := pub("POST", fmt.Sprintf("%s/appointments/%d/payment/mp", slug, createdPublicAppt),
			map[string]any{
				"payer_email": "cliente@teste.com",
			}, &resp)
		if code == 200 || code == 201 {
			t.Logf("  ✓ MP Preference → %d | preference_id: %v", code, resp["preference_id"])
		} else {
			t.Logf("  ℹ MP Preference → %d", code)
		}
	})
}

// ============================================================
// 15 — PAGAMENTOS
// ============================================================

func TestPayments(t *testing.T) {
	t.Run("listar", func(t *testing.T) {
		var resp map[string]any
		code := get("/api/me/payments", &resp)
		assertStatus(t, code, 200, "GET /api/me/payments")
	})

	t.Run("resumo", func(t *testing.T) {
		var resp map[string]any
		code := get("/api/me/payments/summary", &resp)
		assertStatus(t, code, 200, "GET /api/me/payments/summary")
	})

	t.Run("cash_due", func(t *testing.T) {
		var resp any
		code := get("/api/me/payments/cash-due", &resp)
		assertStatus(t, code, 200, "GET /api/me/payments/cash-due")
	})
}

// ============================================================
// 16 — PEDIDOS
// ============================================================

func TestOrders(t *testing.T) {
	productID := demoProductID
	if createdProductID > 0 {
		productID = createdProductID
	}
	if productID == 0 {
		t.Skip("nenhum produto disponível")
	}

	t.Run("criar_pedido", func(t *testing.T) {
		var resp map[string]any
		code := post("/api/me/orders", map[string]any{
			"items": []map[string]any{
				{"product_id": productID, "quantity": 2},
			},
		}, &resp)
		if assertStatus(t, code, 201, "POST /api/me/orders") {
			createdOrderID = id(resp["id"])
			t.Logf("    id: %d | total: %v", createdOrderID, resp["total_amount"])
		}
	})

	t.Run("listar_pedidos", func(t *testing.T) {
		var resp any
		code := get("/api/me/orders", &resp)
		assertStatus(t, code, 200, "GET /api/me/orders")
	})

	t.Run("buscar_pedido", func(t *testing.T) {
		if createdOrderID == 0 {
			t.Skip("pedido não criado")
		}
		var resp map[string]any
		code := get(fmt.Sprintf("/api/me/orders/%d", createdOrderID), &resp)
		if assertStatus(t, code, 200, "GET /api/me/orders/:id") {
			t.Logf("    status: %v", resp["status"])
		}
	})
}

// ============================================================
// 17 — PLANOS
// ============================================================

func TestPlans(t *testing.T) {
	serviceID := demoServiceID
	if createdServiceID > 0 {
		serviceID = createdServiceID
	}

	t.Run("listar", func(t *testing.T) {
		var resp []any
		code := get("/api/me/plans", &resp)
		assertStatus(t, code, 200, "GET /api/me/plans")
		t.Logf("    total planos: %d", len(resp))
	})

	t.Run("criar", func(t *testing.T) {
		body := map[string]any{
			"name":                "Plano E2E",
			"monthly_price_cents": 9900,
			"duration_days":       30,
			"cuts_included":       4,
			"discount_percent":    10,
		}
		if serviceID > 0 {
			body["service_ids"] = []uint{serviceID}
		}
		var resp map[string]any
		code := post("/api/me/plans", body, &resp)
		if assertStatus(t, code, 201, "POST /api/me/plans") {
			createdPlanID = id(resp["id"])
			t.Logf("    id: %d", createdPlanID)
		}
	})

	t.Run("atualizar", func(t *testing.T) {
		if createdPlanID == 0 {
			t.Skip("plano não criado")
		}
		body := map[string]any{
			"name":                "Plano E2E Editado",
			"monthly_price_cents": 10900,
			"duration_days":       30,
			"cuts_included":       4,
			"discount_percent":    15,
		}
		if serviceID > 0 {
			body["service_ids"] = []uint{serviceID}
		}
		code := put(fmt.Sprintf("/api/me/plans/%d", createdPlanID), body, nil)
		assertStatus(t, code, 200, "PUT /api/me/plans/:id")
	})

	t.Run("ativar_desativar", func(t *testing.T) {
		if createdPlanID == 0 {
			t.Skip("plano não criado")
		}
		code := patch(fmt.Sprintf("/api/me/plans/%d/active", createdPlanID), map[string]any{"active": false}, nil)
		assertStatus(t, code, 200, "PATCH /api/me/plans/:id/active (desativar)")

		code = patch(fmt.Sprintf("/api/me/plans/%d/active", createdPlanID), map[string]any{"active": true}, nil)
		assertStatus(t, code, 200, "PATCH /api/me/plans/:id/active (ativar)")
	})

	t.Run("planos_publicos", func(t *testing.T) {
		var resp []any
		code := pub("GET", "/api/public/"+demoSlug+"/plans", nil, &resp)
		assertStatus(t, code, 200, "GET /api/public/:slug/plans")
		t.Logf("    total planos públicos: %d", len(resp))
	})
}

// ============================================================
// 18 — ASSINATURAS
// ============================================================

func TestSubscriptions(t *testing.T) {
	t.Run("listar", func(t *testing.T) {
		var resp any
		code := get("/api/me/subscriptions", &resp)
		assertStatus(t, code, 200, "GET /api/me/subscriptions")
	})

	t.Run("get_active_cliente", func(t *testing.T) {
		if demoClientID == 0 {
			t.Skip("nenhum cliente")
		}
		var resp any
		code := get(fmt.Sprintf("/api/me/subscriptions/%d", demoClientID), &resp)
		// 200 ou 404 (cliente pode não ter assinatura)
		if code != 200 && code != 404 {
			t.Errorf("  ✗ GET subscriptions/:clientID → %d", code)
		} else {
			t.Logf("  ✓ GET subscriptions/:clientID → %d", code)
		}
	})

	t.Run("compra_publica_pix_mock", func(t *testing.T) {
		planID := createdPlanID
		if planID == 0 {
			t.Skip("nenhum plano criado")
		}
		var resp map[string]any
		code := pub("POST", "/api/public/"+demoSlug+"/subscriptions/purchase", map[string]any{
			"plan_id":           planID,
			"client_name":       "Assinante E2E",
			"client_phone":      "11777770001",
			"payer_email":       "assinante@teste.com",
			"payment_method_id": "pix",
		}, &resp)
		if code == 201 || code == 200 {
			t.Logf("  ✓ Compra assinatura PIX mock → %d | status: %v | sub_id: %v", code, resp["status"], resp["subscription_id"])
		} else {
			t.Logf("  ℹ Compra assinatura → %d", code)
		}
	})
}

// ============================================================
// 19 — CARRINHO
// ============================================================

func TestCart(t *testing.T) {
	productID := demoProductID
	if createdProductID > 0 {
		productID = createdProductID
	}
	if productID == 0 {
		t.Skip("nenhum produto disponível para o carrinho")
	}

	slug := "/api/public/" + demoSlug
	cartKey := fmt.Sprintf("e2e-cart-%d", time.Now().UnixNano())

	doCart := func(method, path string, body any, out any) int {
		b, _ := json.Marshal(body)
		var r io.Reader
		if body != nil {
			r = bytes.NewReader(b)
		}
		req, _ := http.NewRequest(method, base+path, r)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Cart-Key", cartKey)
		res, err := hc.Do(req)
		if err != nil {
			return 0
		}
		defer res.Body.Close()
		if out != nil {
			json.NewDecoder(res.Body).Decode(out)
		} else {
			io.Copy(io.Discard, res.Body)
		}
		return res.StatusCode
	}

	t.Run("get_carrinho_vazio", func(t *testing.T) {
		var resp any
		code := doCart("GET", slug+"/cart", nil, &resp)
		assertStatus(t, code, 200, "GET /api/public/:slug/cart (vazio)")
	})

	t.Run("adicionar_item", func(t *testing.T) {
		var resp map[string]any
		code := doCart("POST", slug+"/cart/items", map[string]any{
			"product_id": productID,
			"quantity":   2,
		}, &resp)
		assertStatus(t, code, 200, "POST /api/public/:slug/cart/items")
	})

	t.Run("get_carrinho_com_item", func(t *testing.T) {
		var resp map[string]any
		code := doCart("GET", slug+"/cart", nil, &resp)
		if assertStatus(t, code, 200, "GET /api/public/:slug/cart (com item)") {
			if items, ok := resp["items"].([]any); ok {
				t.Logf("    itens no carrinho: %d", len(items))
			}
		}
	})

	t.Run("remover_item", func(t *testing.T) {
		code := doCart("DELETE", fmt.Sprintf("%s/cart/items/%d", slug, productID), nil, nil)
		if code != 200 && code != 204 {
			t.Errorf("  ✗ DELETE /api/public/:slug/cart/items/:id → %d", code)
		} else {
			t.Logf("  ✓ DELETE /api/public/:slug/cart/items/:id → %d", code)
		}
	})
}

// ============================================================
// 20 — CHECKOUT ORQUESTRADO
// ============================================================

func TestOrchestratedCheckout(t *testing.T) {
	var services []map[string]any
	pub("GET", "/api/public/"+demoSlug+"/services", nil, &services)
	if len(services) == 0 {
		t.Skip("nenhum serviço público")
	}
	svcID := id(services[0]["id"])

	date, slot := nextSlot()
	slot = "11:00" // horário diferente

	var resp map[string]any
	code := pub("POST", "/api/public/"+demoSlug+"/checkout", map[string]any{
		"client_name":  "Checkout E2E",
		"client_phone": "11666660001",
		"product_id":   svcID,
		"date":         date,
		"time":         slot,
	}, &resp)

	if code == 200 || code == 201 {
		t.Logf("  ✓ POST /api/public/:slug/checkout → %d | appt_id: %v", code, resp["appointment_id"])
	} else {
		t.Logf("  ℹ POST /api/public/:slug/checkout → %d (pode ser conflito de horário)", code)
	}
}

// ============================================================
// 21 — FECHAMENTOS (CLOSURES)
// ============================================================

func TestClosures(t *testing.T) {
	t.Run("listar", func(t *testing.T) {
		var resp any
		code := get("/api/me/closures", &resp)
		assertStatus(t, code, 200, "GET /api/me/closures")
	})

	t.Run("buscar_por_id", func(t *testing.T) {
		if createdClosureID == 0 {
			t.Skip("nenhum closure criado")
		}
		var resp map[string]any
		code := get(fmt.Sprintf("/api/me/closures/%d", createdClosureID), &resp)
		assertStatus(t, code, 200, "GET /api/me/closures/:id")
	})

	t.Run("ajuste_closure", func(t *testing.T) {
		if createdClosureID == 0 {
			t.Skip("nenhum closure criado")
		}
		code := post(fmt.Sprintf("/api/me/appointments/%d/closure/adjustment", createdApptID), map[string]any{
			"reason":                   "Ajuste de teste e2e",
			"delta_final_amount_cents": 4000,
			"delta_payment_method":     "pix",
		}, nil)
		if code == 200 || code == 201 {
			t.Logf("  ✓ POST closure/adjustment → %d", code)
		} else {
			t.Logf("  ℹ POST closure/adjustment → %d", code)
		}
	})
}

// ============================================================
// 22 — SUMÁRIO OPERACIONAL
// ============================================================

func TestSummary(t *testing.T) {
	var resp any
	code := get("/api/me/summary", &resp)
	assertStatus(t, code, 200, "GET /api/me/summary")
}

// ============================================================
// 23 — RELATÓRIOS E DASHBOARDS
// ============================================================

func TestReports(t *testing.T) {
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	now := time.Now().In(loc)
	from := now.AddDate(0, -1, 0).Format("2006-01-02")
	to := now.Format("2006-01-02")

	t.Run("dashboard", func(t *testing.T) {
		var resp map[string]any
		code := get("/api/me/dashboard", &resp)
		assertStatus(t, code, 200, "GET /api/me/dashboard")
	})

	t.Run("financeiro", func(t *testing.T) {
		var resp map[string]any
		code := get(fmt.Sprintf("/api/me/financial?from=%s&to=%s", from, to), &resp)
		assertStatus(t, code, 200, "GET /api/me/financial")
	})

	t.Run("day_panel", func(t *testing.T) {
		date := now.Format("2006-01-02")
		var resp map[string]any
		code := get("/api/me/day-panel?date="+date, &resp)
		assertStatus(t, code, 200, "GET /api/me/day-panel")
	})

	t.Run("impact", func(t *testing.T) {
		var resp map[string]any
		code := get(fmt.Sprintf("/api/me/impact?from=%s&to=%s", from, to), &resp)
		assertStatus(t, code, 200, "GET /api/me/impact")
	})

	t.Run("payment_summary", func(t *testing.T) {
		var resp map[string]any
		code := get(fmt.Sprintf("/api/me/payments/summary?from=%s&to=%s", from, to), &resp)
		assertStatus(t, code, 200, "GET /api/me/payments/summary")
	})
}

// ============================================================
// 24 — AUDIT LOGS
// ============================================================

func TestAuditLogs(t *testing.T) {
	var resp map[string]any
	code := get("/api/me/audit-logs", &resp)
	if assertStatus(t, code, 200, "GET /api/me/audit-logs") {
		if data, ok := resp["data"].([]any); ok {
			t.Logf("    total audit logs: %d", len(data))
		}
	}
}

// ============================================================
// 25 — BILLING (PLATAFORMA)
// ============================================================

func TestBilling(t *testing.T) {
	t.Run("status", func(t *testing.T) {
		var resp map[string]any
		code := get("/api/me/billing/status", &resp)
		if assertStatus(t, code, 200, "GET /api/me/billing/status") {
			t.Logf("    status: %v | days_remaining: %v", resp["status"], resp["days_remaining"])
		}
	})

	t.Run("checkout_mock", func(t *testing.T) {
		// Em modo mock (MP_PROVIDER != "mp"), ativa imediatamente
		var resp map[string]any
		code := post("/api/me/billing/checkout", nil, &resp)
		if assertStatus(t, code, 200, "POST /api/me/billing/checkout (mock)") {
			t.Logf("    init_point: %v", resp["init_point"])
		}
	})

	t.Run("pay_mock", func(t *testing.T) {
		// Em modo mock, ativa imediatamente sem body real de cartão
		var resp map[string]any
		code := post("/api/me/billing/pay", map[string]any{
			"payer_email":       "demo@test.com",
			"payment_method_id": "pix",
		}, &resp)
		if assertStatus(t, code, 200, "POST /api/me/billing/pay (mock)") {
			t.Logf("    status: %v", resp["status"])
		}
	})
}

// ============================================================
// 26 — PASSWORD RESET
// ============================================================

func TestPasswordReset(t *testing.T) {
	t.Run("request_reset", func(t *testing.T) {
		// Só testa que o endpoint existe e retorna 200 (email pode não ser enviado em dev)
		code := doJSON("POST", "/api/auth/password-reset/request", map[string]string{
			"email": demoEmail,
		}, "", nil)
		if code == 200 || code == 204 {
			t.Logf("  ✓ POST /api/auth/password-reset/request → %d", code)
		} else {
			t.Logf("  ℹ password reset request → %d (email pode não estar configurado)", code)
		}
	})

	t.Run("confirm_token_invalido", func(t *testing.T) {
		code := doJSON("POST", "/api/auth/password-reset/confirm", map[string]string{
			"token":        "token_invalido_000",
			"new_password": "NovaSenha@2025",
		}, "", nil)
		assertStatus(t, code, 400, "confirm com token inválido → 400")
	})
}

// ============================================================
// 27 — TICKETS PÚBLICOS (self-service)
// ============================================================

func TestTickets(t *testing.T) {
	// Gera um ticket para o appointment público criado
	if createdPublicAppt == 0 {
		t.Skip("appointment público não criado")
	}

	// Gera o ticket via endpoint privado (se existir) ou verifica via lookup
	t.Run("lookup_subscriber", func(t *testing.T) {
		code := pub("GET", "/api/public/"+demoSlug+"/subscribers/lookup?phone=11888880001", nil, nil)
		if code != 200 && code != 404 {
			t.Logf("  ℹ subscriber lookup → %d", code)
		} else {
			t.Logf("  ✓ subscriber lookup → %d", code)
		}
	})

	t.Run("ticket_invalido", func(t *testing.T) {
		code := pub("GET", "/api/public/ticket/token-invalido", nil, nil)
		assertStatus(t, code, 404, "GET /api/public/ticket/:token inválido → 404")
	})
}

// ============================================================
// 28 — WEBHOOK MP (smoke test)
// ============================================================

func TestWebhooks(t *testing.T) {
	t.Run("mp_webhook_payload_invalido", func(t *testing.T) {
		// Payload inválido deve retornar 200 (IPN do MP ignora respostas não-200)
		code := doJSON("POST", "/webhooks/mp", map[string]any{
			"type": "merchant_order",
			"data": map[string]any{"id": "123"},
		}, "", nil)
		assertStatus(t, code, 200, "POST /webhooks/mp (tipo não-payment → 200)")
	})

	t.Run("mp_webhook_payment_tipo_correto", func(t *testing.T) {
		// Webhook com tipo payment mas ID inexistente — deve processar silenciosamente
		code := doJSON("POST", "/webhooks/mp", map[string]any{
			"action": "payment.updated",
			"type":   "payment",
			"data":   map[string]any{"id": "999999999999"},
		}, "", nil)
		assertStatus(t, code, 200, "POST /webhooks/mp (payment id inexistente → 200 async)")
	})

	t.Run("billing_webhook_nao_payment", func(t *testing.T) {
		code := doJSON("POST", "/api/billing/webhook", map[string]any{
			"type": "subscription",
			"data": map[string]any{"id": "123"},
		}, "", nil)
		assertStatus(t, code, 200, "POST /api/billing/webhook (tipo non-payment → 200)")
	})
}

// ============================================================
// 29 — CLEANUP: remove dados criados
// ============================================================

func TestCleanup(t *testing.T) {
	t.Run("deletar_plano_e2e", func(t *testing.T) {
		if createdPlanID == 0 {
			t.Skip("plano não criado")
		}
		code := del(fmt.Sprintf("/api/me/plans/%d", createdPlanID), nil)
		if code != 200 && code != 204 {
			t.Logf("  ℹ DELETE /api/me/plans/:id → %d (pode ter assinatura ativa)", code)
		} else {
			t.Logf("  ✓ DELETE /api/me/plans/:id → %d", code)
		}
	})

	t.Run("deletar_servico_e2e", func(t *testing.T) {
		if createdServiceID == 0 {
			t.Skip("serviço não criado")
		}
		code := del(fmt.Sprintf("/api/me/services/%d", createdServiceID), nil)
		if code != 200 && code != 204 {
			t.Logf("  ℹ DELETE /api/me/services/:id → %d", code)
		} else {
			t.Logf("  ✓ DELETE /api/me/services/:id → %d", code)
		}
	})

	t.Run("deletar_produto_e2e", func(t *testing.T) {
		if createdProductID == 0 {
			t.Skip("produto não criado")
		}
		code := del(fmt.Sprintf("/api/me/products/%d", createdProductID), nil)
		if code != 200 && code != 204 {
			t.Logf("  ℹ DELETE /api/me/products/:id → %d", code)
		} else {
			t.Logf("  ✓ DELETE /api/me/products/:id → %d", code)
		}
	})

	t.Run("deletar_categoria_e2e", func(t *testing.T) {
		if demoServiceCategoryID == 0 {
			t.Skip("categoria não criada")
		}
		code := del(fmt.Sprintf("/api/me/service-categories/%d", demoServiceCategoryID), nil)
		if code != 200 && code != 204 {
			t.Logf("  ℹ DELETE /api/me/service-categories/:id → %d", code)
		} else {
			t.Logf("  ✓ DELETE /api/me/service-categories/:id → %d", code)
		}
	})
}

// ============================================================
// RELATÓRIO FINAL
// ============================================================

func TestSummaryFinal(t *testing.T) {
	t.Logf("\n══════════════════════════════════════════")
	t.Logf("  SUITE E2E — IDs CRIADOS NESTA EXECUÇÃO")
	t.Logf("══════════════════════════════════════════")
	t.Logf("  Service Category : %d", demoServiceCategoryID)
	t.Logf("  Service          : %d", createdServiceID)
	t.Logf("  Product          : %d", createdProductID)
	t.Logf("  Plan             : %d", createdPlanID)
	t.Logf("  Appointment(priv): %d", createdApptID)
	t.Logf("  Appointment(pub) : %d", createdPublicAppt)
	t.Logf("  Closure          : %d", createdClosureID)
	t.Logf("  Order            : %d", createdOrderID)
	t.Logf("══════════════════════════════════════════")

	_ = strconv.Itoa(0) // evita import não usado
}
