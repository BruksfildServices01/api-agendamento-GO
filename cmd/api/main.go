package main

import (
	"log"
	"net/http"

	"github.com/BruksfildServices01/barber-scheduler/internal/config"
	dbpkg "github.com/BruksfildServices01/barber-scheduler/internal/db"
	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/routes"
	"github.com/gin-gonic/gin"
)

func main() {

	cfg := config.Load()
	db := dbpkg.NewDB(cfg)

	if gin.Mode() == gin.ReleaseMode {
		gin.SetMode(gin.ReleaseMode)
	}

	// 🚀 Router enxuto (sem logger por padrão)
	r := gin.New()
	r.Use(gin.Recovery())

	// 🧾 Logger só em desenvolvimento
	if gin.Mode() != gin.ReleaseMode {
		r.Use(gin.Logger())
	}

	// 🌍 CORS
	r.Use(middleware.CORSMiddleware())

	// ❤️ Healthcheck
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 🧩 Rotas
	routes.RegisterRoutes(r, db, cfg)

	log.Printf("Server running on %s", cfg.Addr())
	if err := r.Run(cfg.Addr()); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
