package routes

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	"github.com/BruksfildServices01/barber-scheduler/internal/config"
	"github.com/BruksfildServices01/barber-scheduler/internal/handlers"
	infraRepo "github.com/BruksfildServices01/barber-scheduler/internal/infra/repository"
	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	ucAppointment "github.com/BruksfildServices01/barber-scheduler/internal/usecase/appointment"
)

func RegisterRoutes(r *gin.Engine, db *gorm.DB, cfg *config.Config) {

	// ======================================================
	// 🌍 MIDDLEWARE GLOBAL
	// ======================================================
	r.Use(middleware.CORSMiddleware())

	// ======================================================
	// 🔧 INFRA (SINGLETONS)
	// ======================================================
	appointmentRepo := infraRepo.NewAppointmentGormRepository(db)

	auditLogger := audit.New(db)
	auditDispatcher := audit.NewDispatcher(auditLogger)

	// ======================================================
	// 🧠 USE CASES — APPOINTMENTS
	// ======================================================
	createAppointmentUC := ucAppointment.NewCreatePrivateAppointment(
		appointmentRepo,
		auditDispatcher,
	)

	completeAppointmentUC := ucAppointment.NewCompleteAppointment(
		appointmentRepo,
		auditDispatcher,
	)

	cancelAppointmentUC := ucAppointment.NewCancelAppointment(
		appointmentRepo,
		auditDispatcher,
	)

	listAppointmentsByDateUC := ucAppointment.NewListAppointmentsByDate(
		appointmentRepo,
	)

	listAppointmentsByMonthUC := ucAppointment.NewListAppointmentsByMonth(
		appointmentRepo,
	)

	// ======================================================
	// 🧩 HANDLERS
	// ======================================================
	authHandler := handlers.NewAuthHandler(db, cfg)
	meHandler := handlers.NewMeHandler(db)
	barbershopHandler := handlers.NewBarbershopHandler(db)

	barberProductHandler := handlers.NewBarberProductHandler(db)
	clientHandler := handlers.NewClientHandler(db)
	workingHoursHandler := handlers.NewWorkingHoursHandler(db)

	appointmentHandler := handlers.NewAppointmentHandler(
		createAppointmentUC,
		completeAppointmentUC,
		cancelAppointmentUC,
		listAppointmentsByDateUC,
		listAppointmentsByMonthUC, // ✅ FALTAVA ISSO
	)

	auditLogsHandler := handlers.NewAuditLogsHandler(db)

	publicHandler := handlers.NewPublicHandler(db)
	publicWebHandler := handlers.NewPublicWebHandler(db)
	appWebHandler := handlers.NewAppWebHandler(db)

	// ======================================================
	// 🌍 ROTAS WEB (HTML)
	// ======================================================
	r.GET("/web/public/:slug", publicWebHandler.ShowBookingPage)

	webApp := r.Group("/web/app")
	{
		webApp.GET("/login", appWebHandler.LoginPage)
		webApp.GET("/dashboard", appWebHandler.Dashboard)
		webApp.GET("/services", appWebHandler.Services)
	}

	// ======================================================
	// 🌐 API (JSON)
	// ======================================================
	api := r.Group("/api")
	{
		// ------------------------------
		// 🌐 API PÚBLICA
		// ------------------------------
		publicAPI := api.Group("/public")
		{
			publicAPI.GET("/:slug/products", publicHandler.ListProducts)
			publicAPI.GET("/:slug/availability", publicHandler.AvailabilityForClient)
			publicAPI.POST("/:slug/appointments", publicHandler.CreateAppointment)
		}

		// ------------------------------
		// 🔐 AUTH
		// ------------------------------
		api.POST("/auth/register", authHandler.Register)
		api.POST("/auth/login", authHandler.Login)

		// ------------------------------
		// 🔐 API PRIVADA
		// ------------------------------
		secured := api.Group("/")
		secured.Use(middleware.AuthMiddleware(cfg))
		{
			secured.GET("/me", meHandler.GetMe)

			secured.GET("/me/barbershop", barbershopHandler.GetMeBarbershop)
			secured.PATCH("/me/barbershop", barbershopHandler.UpdateMeBarbershop)

			secured.GET("/me/clients", clientHandler.List)

			secured.GET("/me/products", barberProductHandler.List)
			secured.POST("/me/products", barberProductHandler.Create)
			secured.PATCH("/me/products/:id", barberProductHandler.Update)

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

			secured.GET("/me/audit-logs", auditLogsHandler.List)
		}
	}
}
