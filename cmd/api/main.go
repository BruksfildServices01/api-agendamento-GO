package main

import (
	"context"
	"log"
	"net/http"
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
	db := dbpkg.NewDB(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Scheduler único
	scheduler := jobs.NewScheduler(ctx)

	// Gin
	if gin.Mode() == gin.ReleaseMode {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())

	if gin.Mode() != gin.ReleaseMode {
		r.Use(gin.Logger())
	}

	r.Use(middleware.CORSMiddleware(cfg.CORSAllowedOrigins))

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// ✅ Agora routes também registra os jobs (sem duplicação)
	routes.RegisterRoutes(r, db, cfg, scheduler)

	log.Printf("Server running on %s", cfg.Addr())

	if err := r.Run(cfg.Addr()); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
