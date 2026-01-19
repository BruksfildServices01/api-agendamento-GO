package routes

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/config"
	"github.com/BruksfildServices01/barber-scheduler/internal/handlers"
	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
)

func RegisterRoutes(r *gin.Engine, db *gorm.DB, cfg *config.Config) {

	// ======================================================
	// 🌍 MIDDLEWARE GLOBAL
	// ======================================================
	r.Use(middleware.CORSMiddleware())

	// ======================================================
	// 🧩 HANDLERS
	// ======================================================
	authHandler := handlers.NewAuthHandler(db, cfg)
	meHandler := handlers.NewMeHandler(db)
	barbershopHandler := handlers.NewBarbershopHandler(db)

	barberProductHandler := handlers.NewBarberProductHandler(db)
	clientHandler := handlers.NewClientHandler(db)
	workingHoursHandler := handlers.NewWorkingHoursHandler(db)
	appointmentHandler := handlers.NewAppointmentHandler(db)
	auditLogsHandler := handlers.NewAuditLogsHandler(db)

	publicHandler := handlers.NewPublicHandler(db)

	publicWebHandler := handlers.NewPublicWebHandler(db)
	appWebHandler := handlers.NewAppWebHandler(db)

	// ======================================================
	// 🌍 ROTAS WEB (HTML) — SEM AUTH
	// ======================================================
	r.GET("/web/public/:slug", publicWebHandler.ShowBookingPage)

	webApp := r.Group("/web/app")
	{
		webApp.GET("/login", appWebHandler.LoginPage)
		webApp.GET("/dashboard", appWebHandler.Dashboard)
		webApp.GET("/services", appWebHandler.Services)
	}

	// DEV (opcional)
	r.GET("/web/dev/services", appWebHandler.Services)

	// ======================================================
	// 🌐 API (JSON)
	// ======================================================
	api := r.Group("/api")
	{
		// ==================================================
		// 🌐 API PÚBLICA — SEM AUTH
		// ==================================================
		publicAPI := api.Group("/public")
		{
			publicAPI.GET("/:slug/products", publicHandler.ListProducts)
			publicAPI.GET("/:slug/availability", publicHandler.AvailabilityForClient)
			publicAPI.POST("/:slug/appointments", publicHandler.CreateAppointment)
		}

		// ==================================================
		// 🔐 AUTH
		// ==================================================
		api.POST("/auth/register", authHandler.Register)
		api.POST("/auth/login", authHandler.Login)

		// ==================================================
		// 🔐 API PRIVADA — COM JWT
		// ==================================================
		secured := api.Group("/")
		secured.Use(middleware.AuthMiddleware(cfg))
		{
			// ------------------------------
			// ME
			// ------------------------------
			secured.GET("/me", meHandler.GetMe)

			secured.GET("/me/barbershop", barbershopHandler.GetMeBarbershop)
			secured.PATCH("/me/barbershop", barbershopHandler.UpdateMeBarbershop)

			// ------------------------------
			// CLIENTS
			// ------------------------------
			secured.GET("/me/clients", clientHandler.List)

			// ------------------------------
			// PRODUCTS
			// ------------------------------
			secured.GET("/me/products", barberProductHandler.List)
			secured.POST("/me/products", barberProductHandler.Create)
			secured.PATCH("/me/products/:id", barberProductHandler.Update)

			// ------------------------------
			// WORKING HOURS
			// ------------------------------
			secured.GET("/me/working-hours", workingHoursHandler.Get)
			secured.PUT("/me/working-hours", workingHoursHandler.Update)

			// ------------------------------
			// APPOINTMENTS
			// ------------------------------
			secured.POST("/me/appointments", appointmentHandler.Create)
			secured.GET("/me/appointments", appointmentHandler.ListByDate)
			secured.GET("/me/appointments/month", appointmentHandler.ListByMonth)
			secured.PATCH("/me/appointments/:id/cancel", appointmentHandler.Cancel)
			secured.PATCH("/me/appointments/:id/complete", appointmentHandler.Complete)

			// ------------------------------
			// AUDIT LOGS
			// ------------------------------
			secured.GET("/me/audit-logs", auditLogsHandler.List)
		}
	}
}
