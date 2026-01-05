package main

import (
	"log"

	"github.com/gin-gonic/gin"

	"github.com/BruksfildServices01/barber-scheduler/internal/config"
	dbpkg "github.com/BruksfildServices01/barber-scheduler/internal/db"
	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/routes"
)

func main() {
	cfg := config.Load()
	db := dbpkg.NewDB(cfg)

	r := gin.Default()

	// 🔹 Carrega templates HTML (Gin)
	r.LoadHTMLGlob("templates/**/*.html")

	// 🔹 Arquivos estáticos (CSS, JS, imagens)
	r.Static("/static", "./static")

	// 🔹 CORS (para chamadas JS)
	r.Use(middleware.CORSMiddleware())

	// 🔹 Healthcheck
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// 🔹 Rotas do sistema
	routes.RegisterRoutes(r, db, cfg)

	log.Printf("Server running on %s", cfg.Addr())
	if err := r.Run(cfg.Addr()); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
