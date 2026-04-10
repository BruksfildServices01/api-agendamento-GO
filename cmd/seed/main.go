// cmd/seed/main.go — Demo seed for presentation
// Usage: go run ./cmd/seed
package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"os"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	dbpkg "github.com/BruksfildServices01/barber-scheduler/internal/db"
	"github.com/BruksfildServices01/barber-scheduler/internal/config"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

func ptr[T any](v T) *T { return &v }

func main() {
	_ = godotenv.Load()
	cfg := &config.Config{
		DBUrl:     os.Getenv("DATABASE_URL"),
		JWTSecret: os.Getenv("JWT_SECRET"),
	}
	db := dbpkg.NewDB(cfg)

	rng := rand.New(rand.NewSource(42))

	// ── 1. Barbershop + owner ─────────────────────────────────────────────────
	hash, _ := bcrypt.GenerateFromPassword([]byte("Demo@2025"), bcrypt.DefaultCost)

	shop := models.Barbershop{
		Name:              "Barbearia Prime",
		Slug:              "barbearia-prime",
		Phone:             "(11) 99876-5432",
		Address:           "Rua Augusta, 1245 – Consolação, São Paulo – SP",
		MinAdvanceMinutes: 60,
		Timezone:          "America/Sao_Paulo",
	}
	if err := db.Create(&shop).Error; err != nil {
		log.Fatal("create barbershop:", err)
	}

	owner := models.User{
		BarbershopID: &shop.ID,
		Name:         "Rafael Mendes",
		Email:        "demo@barbeariprime.com.br",
		PasswordHash: string(hash),
		Phone:        "(11) 99876-5432",
		Role:         "owner",
	}
	if err := db.Create(&owner).Error; err != nil {
		log.Fatal("create user:", err)
	}
	log.Printf("✓ Barbearia: %s  |  Login: %s / Demo@2025", shop.Name, owner.Email)

	// ── 2. Payment config ─────────────────────────────────────────────────────
	db.Create(&models.BarbershopPaymentConfig{
		BarbershopID:         shop.ID,
		DefaultRequirement:   "mandatory",
		PixExpirationMinutes: 15,
		AcceptCash:           true,
		AcceptPix:            true,
		AcceptCredit:         true,
		AcceptDebit:          true,
	})

	// ── 3. Working hours (seg-sáb, 9h–20h, almoço 12h–13h) ──────────────────
	for wd := 0; wd <= 6; wd++ {
		wh := models.WorkingHours{
			BarbershopID: shop.ID,
			BarberID:     0,
			Weekday:      wd,
			Active:       wd != 0, // domingo fechado
			StartTime:    "09:00",
			EndTime:      "20:00",
			LunchStart:   "12:00",
			LunchEnd:     "13:00",
		}
		db.Create(&wh)
	}

	// ── 4. Service categories ─────────────────────────────────────────────────
	catCorte := models.ServiceCategory{BarbershopID: shop.ID, Name: "Corte"}
	catBarba := models.ServiceCategory{BarbershopID: shop.ID, Name: "Barba"}
	catTrat := models.ServiceCategory{BarbershopID: shop.ID, Name: "Tratamento"}
	db.Create(&catCorte)
	db.Create(&catBarba)
	db.Create(&catTrat)

	// ── 5. Services ───────────────────────────────────────────────────────────
	type svcDef struct {
		name string
		desc string
		dur  int
		price int64
		catID uint
	}
	svcs := []svcDef{
		{"Corte Clássico", "Corte masculino com acabamento na navalha", 45, 4000, catCorte.ID},
		{"Corte + Barba", "Combo completo: corte e barba modelada", 75, 6500, catCorte.ID},
		{"Degradê Americano", "Fade progressivo com máquina e tesoura", 50, 4500, catCorte.ID},
		{"Barba Completa", "Barba modelada com toalha quente e óleo", 30, 3500, catBarba.ID},
		{"Barba Express", "Acabamento rápido de barba", 20, 2500, catBarba.ID},
		{"Hidratação Capilar", "Máscara nutritiva + secagem profissional", 45, 5000, catTrat.ID},
	}
	services := make([]*models.BarbershopService, len(svcs))
	for i, s := range svcs {
		svc := &models.BarbershopService{
			BarbershopID: shop.ID,
			Name:         s.name,
			Description:  s.desc,
			DurationMin:  s.dur,
			Price:        s.price,
			Active:       true,
			CategoryID:   &s.catID,
		}
		db.Create(svc)
		services[i] = svc
	}
	log.Printf("✓ %d serviços criados", len(services))

	// ── 6. Products ───────────────────────────────────────────────────────────
	type prodDef struct {
		name  string
		desc  string
		cat   string
		price int64
		stock int
	}
	prods := []prodDef{
		{"Pomada Modeladora Matte", "Fixação forte, acabamento fosco", "Styling", 4800, 24},
		{"Óleo de Barba Premium", "Hidrata e amacia os pelos da barba", "Barba", 3800, 18},
		{"Shampoo Antiqueda", "Fórmula com biotina e queratina", "Cabelo", 4200, 20},
		{"Condicionador Deep", "Hidratação profunda para cabelos secos", "Cabelo", 3500, 15},
		{"Cera Finalizadora", "Brilho intenso e fixação leve", "Styling", 5500, 12},
		{"Balm para Barba", "Hidrata e define a barba com aroma amadeirado", "Barba", 4400, 16},
	}
	products := make([]*models.Product, len(prods))
	for i, p := range prods {
		prod := &models.Product{
			BarbershopID:  shop.ID,
			Name:          p.name,
			Description:   p.desc,
			Category:      p.cat,
			Price:         p.price,
			Stock:         p.stock,
			Active:        true,
			OnlineVisible: true,
		}
		db.Create(prod)
		products[i] = prod
	}
	log.Printf("✓ %d produtos criados", len(products))

	// ── 7. Service → Product suggestions ─────────────────────────────────────
	// Corte → Pomada, Cera
	db.Create(&models.ServiceSuggestedProduct{BarbershopID: shop.ID, ServiceID: services[0].ID, ProductID: products[0].ID, Active: true})
	db.Create(&models.ServiceSuggestedProduct{BarbershopID: shop.ID, ServiceID: services[0].ID, ProductID: products[4].ID, Active: true})
	// Corte+Barba → Óleo de Barba
	db.Create(&models.ServiceSuggestedProduct{BarbershopID: shop.ID, ServiceID: services[1].ID, ProductID: products[1].ID, Active: true})
	// Degradê → Pomada
	db.Create(&models.ServiceSuggestedProduct{BarbershopID: shop.ID, ServiceID: services[2].ID, ProductID: products[0].ID, Active: true})
	// Barba → Óleo, Balm
	db.Create(&models.ServiceSuggestedProduct{BarbershopID: shop.ID, ServiceID: services[3].ID, ProductID: products[1].ID, Active: true})
	db.Create(&models.ServiceSuggestedProduct{BarbershopID: shop.ID, ServiceID: services[3].ID, ProductID: products[5].ID, Active: true})
	// Hidratação → Shampoo, Condicionador
	db.Create(&models.ServiceSuggestedProduct{BarbershopID: shop.ID, ServiceID: services[5].ID, ProductID: products[2].ID, Active: true})
	db.Create(&models.ServiceSuggestedProduct{BarbershopID: shop.ID, ServiceID: services[5].ID, ProductID: products[3].ID, Active: true})

	// ── 8. Clients ────────────────────────────────────────────────────────────
	clientNames := []string{
		"Lucas Oliveira", "Pedro Alves", "Mateus Costa", "Gabriel Silva", "Felipe Santos",
		"Rodrigo Lima", "Bruno Ferreira", "Diego Souza", "André Martins", "Carlos Pereira",
		"Rafael Carvalho", "Thiago Rocha", "Leonardo Melo", "Gustavo Ribeiro", "Eduardo Nunes",
		"Henrique Araújo", "Vinicius Cardoso", "Alexandre Barros", "Renato Castro", "Marcelo Dias",
		"Paulo Moreira", "João Mendes", "Igor Nascimento", "Caio Teixeira", "Leandro Gomes",
		"Samuel Pinto", "Fabio Cavalcanti", "Daniel Freitas", "Marcos Azevedo", "Luiz Correia",
		"Nicolas Torres", "Jonathan Borges", "Renan Cunha", "Alan Campos", "Murilo Fonseca",
		"Victor Lopes", "Filipe Medeiros", "Cleber Rezende", "Danilo Nogueira", "Ricardo Viana",
		"Adriano Machado", "Sérgio Pacheco", "Emerson Batista", "Wagner Monteiro", "Julio Miranda",
	}
	phones := []string{
		"11987654321", "11976543210", "11965432109", "11954321098", "11943210987",
		"11932109876", "11921098765", "11910987654", "11909876543", "11898765432",
		"11887654321", "11876543210", "11865432109", "11854321098", "11843210987",
		"11832109876", "11821098765", "11810987654", "11809876543", "11798765432",
		"11787654321", "11776543210", "11765432109", "11754321098", "11743210987",
		"11732109876", "11721098765", "11710987654", "11709876543", "11698765432",
		"11687654321", "11676543210", "11665432109", "11654321098", "11643210987",
		"11632109876", "11621098765", "11610987654", "11609876543", "11598765432",
		"11587654321", "11576543210", "11565432109", "11554321098", "11543210987",
	}

	clients := make([]*models.Client, len(clientNames))
	for i, name := range clientNames {
		c := &models.Client{
			BarbershopID: &shop.ID,
			Name:         name,
			Phone:        phones[i],
			Email:        fmt.Sprintf("cliente%d@email.com", i+1),
		}
		db.Create(c)
		clients[i] = c
	}
	log.Printf("✓ %d clientes criados", len(clients))

	// ── 9. Appointments + Closures + Payments (últimos 75 dias) ──────────────
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	// Horários disponíveis por slot
	slots := []string{"09:00", "09:50", "10:40", "11:30", "13:00", "13:50", "14:40", "15:30", "16:20", "17:10", "18:00", "18:50"}
	payMethods := []string{"pix", "cash", "card", "pix", "pix", "cash", "card", "pix"}

	totalRevenue := int64(0)
	apptCount := 0
	closureCount := 0

	// Para cada dia dos últimos 75 dias até amanhã
	for dayOffset := -75; dayOffset <= 7; dayOffset++ {
		d := today.AddDate(0, 0, dayOffset)
		wd := int(d.Weekday())
		if wd == 0 { // domingo fechado
			continue
		}

		// 6-12 agendamentos por dia útil
		nAppts := 6 + rng.Intn(7)
		if wd == 6 { // sábado menos movimento
			nAppts = 4 + rng.Intn(4)
		}

		usedSlots := map[string]bool{}
		for a := 0; a < nAppts; a++ {
			// Escolhe slot livre
			var slot string
			for attempts := 0; attempts < 20; attempts++ {
				s := slots[rng.Intn(len(slots))]
				if !usedSlots[s] {
					slot = s
					usedSlots[s] = true
					break
				}
			}
			if slot == "" {
				continue
			}

			// Parse hora
			var hh, mm int
			fmt.Sscanf(slot, "%d:%02d", &hh, &mm)
			startT := time.Date(d.Year(), d.Month(), d.Day(), hh, mm, 0, 0, loc)

			// Serviço aleatório
			svc := services[rng.Intn(len(services))]
			endT := startT.Add(time.Duration(svc.DurationMin) * time.Minute)

			// Cliente aleatório
			client := clients[rng.Intn(len(clients))]

			// Status baseado no dia
			var status models.AppointmentStatus
			var completedAt, cancelledAt, noShowAt *time.Time
			isPast := dayOffset < 0

			if isPast {
				roll := rng.Intn(100)
				switch {
				case roll < 82:
					status = models.AppointmentStatusCompleted
					completedAt = ptr(endT.Add(5 * time.Minute))
				case roll < 90:
					status = models.AppointmentStatusCancelled
					cancelledAt = ptr(startT.Add(-2 * time.Hour))
				default:
					status = models.AppointmentStatusNoShow
					noShowAt = ptr(endT)
				}
			} else {
				status = models.AppointmentStatusScheduled
			}

			appt := models.Appointment{
				BarbershopID:    &shop.ID,
				BarberID:        &owner.ID,
				ClientID:        &client.ID,
				BarberProductID: &svc.ID,
				StartTime:       startT.UTC(),
				EndTime:         endT.UTC(),
				Status:          status,
				CreatedBy:       models.CreatedByClient,
				PaymentIntent:   models.PaymentIntentPayLater,
				CompletedAt:     completedAt,
				CancelledAt:     cancelledAt,
				NoShowAt:        noShowAt,
			}
			if err := db.Create(&appt).Error; err != nil {
				continue
			}
			apptCount++

			// Closure + Payment para completados
			if status == models.AppointmentStatusCompleted {
				payMethod := payMethods[rng.Intn(len(payMethods))]
				amount := svc.Price

				closure := models.AppointmentClosure{
					AppointmentID:          appt.ID,
					BarbershopID:           shop.ID,
					ServiceID:              &svc.ID,
					ServiceName:            svc.Name,
					ReferenceAmountCents:   amount,
					FinalAmountCents:       ptr(amount),
					PaymentMethod:          payMethod,
					RequiresNormalCharging: true,
					ConfirmNormalCharging:  true,
				}
				db.Create(&closure)
				closureCount++

				// Payment
				paidAt := completedAt
				txid := fmt.Sprintf("seed-%d-%d", appt.ID, rng.Int63())
				payment := models.Payment{
					BarbershopID:  shop.ID,
					AppointmentID: &appt.ID,
					TxID:          &txid,
					Amount:        amount,
					Status:        "paid",
					PaidAt:        paidAt,
				}
				db.Create(&payment)
				totalRevenue += amount
			}
		}
	}
	log.Printf("✓ %d agendamentos | %d finalizados | Receita: R$ %.2f", apptCount, closureCount, float64(totalRevenue)/100)

	// ── 10. Standalone product orders (loja) ─────────────────────────────────
	orderCount := 0
	for i := 0; i < 28; i++ {
		daysAgo := rng.Intn(60) + 1
		orderDate := today.AddDate(0, 0, -daysAgo)
		client := clients[rng.Intn(len(clients))]
		prod := products[rng.Intn(len(products))]
		qty := 1 + rng.Intn(2)
		total := prod.Price * int64(qty)

		order := models.Order{
			BarbershopID:   shop.ID,
			ClientID:       &client.ID,
			Type:           models.OrderTypeProduct,
			Status:         models.OrderStatusPaid,
			SubtotalAmount: total,
			TotalAmount:    total,
			CreatedAt:      orderDate,
			UpdatedAt:      orderDate,
		}
		if err := db.Create(&order).Error; err != nil {
			continue
		}

		db.Create(&models.OrderItem{
			OrderID:             order.ID,
			ProductID:           prod.ID,
			ProductNameSnapshot: prod.Name,
			Quantity:            qty,
			UnitPrice:           prod.Price,
			LineTotal:           total,
		})

		txid := fmt.Sprintf("seed-order-%d-%d", order.ID, rng.Int63())
		paidAt := orderDate.Add(5 * time.Minute)
		db.Create(&models.Payment{
			BarbershopID: shop.ID,
			OrderID:      &order.ID,
			TxID:         &txid,
			Amount:       total,
			Status:       "paid",
			PaidAt:       &paidAt,
		})
		orderCount++
	}
	log.Printf("✓ %d pedidos de loja criados", orderCount)

	// ── 11. CRM categories (distribui clientes) ───────────────────────────────
	categories := []string{"trusted", "trusted", "regular", "regular", "regular", "new", "at_risk"}
	for _, c := range clients {
		cat := categories[rng.Intn(len(categories))]
		db.Exec(`INSERT INTO client_crm_categories (barbershop_id, client_id, category, updated_at)
			VALUES (?, ?, ?, NOW())
			ON CONFLICT (barbershop_id, client_id) DO UPDATE SET category = EXCLUDED.category`,
			shop.ID, c.ID, cat)
	}

	fmt.Println()
	fmt.Println("════════════════════════════════════════")
	fmt.Println("  SEED COMPLETO — CONTA DEMO")
	fmt.Println("════════════════════════════════════════")
	fmt.Printf("  URL:   /%s\n", shop.Slug)
	fmt.Printf("  Email: %s\n", owner.Email)
	fmt.Printf("  Senha: Demo@2025\n")
	fmt.Printf("  Clientes: %d\n", len(clients))
	fmt.Printf("  Agendamentos: %d\n", apptCount)
	fmt.Printf("  Finalizados: %d\n", closureCount)
	fmt.Printf("  Pedidos loja: %d\n", orderCount)
	fmt.Printf("  Receita total: R$ %.2f\n", float64(totalRevenue)/100)
	fmt.Println("════════════════════════════════════════")
}

// gorm needs this for seed context
func init() {
	_ = gorm.ErrRecordNotFound
}
