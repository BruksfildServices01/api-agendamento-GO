package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	"github.com/BruksfildServices01/barber-scheduler/internal/config"
	dbpkg "github.com/BruksfildServices01/barber-scheduler/internal/db"
	"github.com/BruksfildServices01/barber-scheduler/internal/jobs"
	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/routes"

	infraRepo "github.com/BruksfildServices01/barber-scheduler/internal/infra/repository"
	ucPayment "github.com/BruksfildServices01/barber-scheduler/internal/usecase/payment"
)

func main() {

	// ======================================================
	// 🌱 LOAD .ENV (DEV / LOCAL)
	// ======================================================
	_ = godotenv.Load()

	// ======================================================
	// ⚙️ CONFIG + DB
	// ======================================================
	cfg := config.Load()
	db := dbpkg.NewDB(cfg)

	// ======================================================
	// 🧠 CONTEXT RAIZ (lifecycle do app)
	// ======================================================
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ======================================================
	// 🧾 AUDIT (compartilhado por HTTP + JOBS)
	// ======================================================
	auditLogger := audit.New(db)
	auditDispatcher := audit.NewDispatcher(auditLogger)

	// ======================================================
	// 🔧 REPOSITORIES (usados por JOBS)
	// ======================================================
	paymentRepo := infraRepo.NewPaymentGormRepository(db)
	appointmentRepo := infraRepo.NewAppointmentGormRepository(db)
	// appointmentRepo implementa:
	// - domain/appointment.Repository
	// - domain/appointment.JobRepository

	// ======================================================
	// 🧠 USE CASE — EXPIRAR PAYMENTS (Sprint 4)
	// ======================================================
	expirePaymentsUC := ucPayment.NewExpirePayments(
		paymentRepo,
		appointmentRepo,
		auditDispatcher,
	)

	// ======================================================
	// ⏱ JOBS + SCHEDULER
	// ======================================================
	expirePaymentsJob := jobs.NewExpirePaymentsJob(
		expirePaymentsUC,
	)

	scheduler := jobs.NewScheduler(ctx)

	// ⏱ Executa a cada 1 minuto (seguro para MVP)
	scheduler.Every(
		time.Minute,
		expirePaymentsJob.Run,
	)

	// ======================================================
	// 🌐 HTTP SERVER (GIN)
	// ======================================================
	if gin.Mode() == gin.ReleaseMode {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())

	// Logger só fora de produção
	if gin.Mode() != gin.ReleaseMode {
		r.Use(gin.Logger())
	}

	r.Use(middleware.CORSMiddleware())

	// ❤️ Healthcheck
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 🧩 Rotas HTTP
	routes.RegisterRoutes(r, db, cfg)

	// ======================================================
	// 🚀 START SERVER
	// ======================================================
	log.Printf("Server running on %s", cfg.Addr())
	if err := r.Run(cfg.Addr()); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
