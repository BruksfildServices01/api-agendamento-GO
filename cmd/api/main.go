package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // embeds timezone database for Alpine containers

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/BruksfildServices01/barber-scheduler/internal/config"
	dbpkg "github.com/BruksfildServices01/barber-scheduler/internal/db"
	"github.com/BruksfildServices01/barber-scheduler/internal/jobs"
	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/routes"
)

func main() {
	_ = godotenv.Load()

	cfg := config.Load()

	// Validações de segurança obrigatórias em produção.
	if cfg.AppEnv == "production" {
		if cfg.MPProvider == "mp" {
			if cfg.MPWebhookSecret == "" {
				log.Fatal("ERRO DE CONFIGURAÇÃO: MP_WEBHOOK_SECRET é obrigatório quando MP_PROVIDER=mp e APP_ENV=production.")
			}
			if cfg.MPAccessToken == "" {
				log.Fatal("ERRO DE CONFIGURAÇÃO: MP_ACCESS_TOKEN é obrigatório quando MP_PROVIDER=mp e APP_ENV=production (necessário para billing da plataforma).")
			}
		}
		if cfg.EvolutionURL != "" && cfg.EvolutionAPIKey == "" {
			log.Fatal("ERRO DE CONFIGURAÇÃO: EVOLUTION_API_KEY é obrigatória quando EVOLUTION_URL está configurada e APP_ENV=production.")
		}
	}

	db := dbpkg.NewDB(cfg)

	// Contexto raiz: cancelado no início do graceful shutdown para parar os jobs.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	scheduler := jobs.NewScheduler(ctx)

	if gin.Mode() == gin.ReleaseMode {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())

	if gin.Mode() != gin.ReleaseMode {
		r.Use(gin.Logger())
	}

	r.Use(middleware.CORSMiddleware(cfg.CORSAllowedOrigins))

	sqlDB, _ := db.DB()
	r.GET("/health", func(c *gin.Context) {
		if err := sqlDB.PingContext(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded", "db": "unreachable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	auditDispatcher := routes.RegisterRoutes(r, db, cfg, scheduler)

	srv := &http.Server{
		Addr:    cfg.Addr(),
		Handler: r,
	}

	// Inicia o servidor em goroutine separada.
	go func() {
		log.Printf("Server running on %s", cfg.Addr())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Aguarda sinal de encerramento (SIGTERM do Kubernetes ou SIGINT do terminal).
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Printf("Received signal %s — shutting down gracefully...", sig)

	// 1. Para os jobs (cancela o contexto do scheduler).
	cancel()

	// 2. Para de aceitar novas requests e aguarda as em andamento (até 30s).
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	// 3. Persiste todos os eventos de auditoria pendentes antes de fechar o DB.
	auditDispatcher.Shutdown()

	log.Println("Server exited cleanly.")
}
