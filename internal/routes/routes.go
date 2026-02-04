package routes

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	"github.com/BruksfildServices01/barber-scheduler/internal/config"
	"github.com/BruksfildServices01/barber-scheduler/internal/handlers"
	"github.com/BruksfildServices01/barber-scheduler/internal/infra/notification"
	"github.com/BruksfildServices01/barber-scheduler/internal/infra/pix"
	infraRepo "github.com/BruksfildServices01/barber-scheduler/internal/infra/repository"
	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"

	domainNotification "github.com/BruksfildServices01/barber-scheduler/internal/domain/notification"

	ucAppointment "github.com/BruksfildServices01/barber-scheduler/internal/usecase/appointment"
	ucPayment "github.com/BruksfildServices01/barber-scheduler/internal/usecase/payment"
	paymentconfig "github.com/BruksfildServices01/barber-scheduler/internal/usecase/paymentconfig"
)

func RegisterRoutes(r *gin.Engine, db *gorm.DB, cfg *config.Config) {

	r.Use(middleware.CORSMiddleware())

	// ======================================================
	// REPOSITORIES
	// ======================================================
	appointmentRepo := infraRepo.NewAppointmentGormRepository(db)
	paymentRepo := infraRepo.NewPaymentGormRepository(db)
	paymentConfigRepo := infraRepo.NewBarbershopPaymentConfigGormRepository(db)

	// ======================================================
	// AUDIT
	// ======================================================
	auditLogger := audit.New(db)
	auditDispatcher := audit.NewDispatcher(auditLogger)

	// ======================================================
	// PIX
	// ======================================================
	pixGateway := pix.NewMockPixGateway()

	// ======================================================
	// NOTIFICATION
	// ======================================================
	var notifier domainNotification.Notifier

	if cfg.EmailEnabled {
		notifier = notification.NewEmailNotifier(cfg)
	} else {
		notifier = notification.NewNoopNotifier()
	}

	// ======================================================
	// PAYMENT CONFIG
	// ======================================================
	resolveBookingPaymentPolicyUC :=
		paymentconfig.NewResolveBookingPaymentPolicy(
			paymentConfigRepo,
		)

	// ======================================================
	// PAYMENT USE CASES
	// ======================================================
	createPixPaymentUC :=
		ucPayment.NewCreatePixPayment(
			paymentRepo,
			pixGateway,
			auditDispatcher,
		)

	markPaymentAsPaidUC :=
		ucPayment.NewMarkPaymentAsPaid(
			paymentRepo,
			appointmentRepo,
			auditDispatcher,
			notifier,
		)

	listPaymentsUC :=
		ucPayment.NewListPaymentsForBarbershop(
			paymentRepo,
		)

	getPaymentSummaryUC :=
		ucPayment.NewGetPaymentSummary(
			paymentRepo,
		)

	// ======================================================
	// APPOINTMENT USE CASES
	// ======================================================
	createAppointmentUC :=
		ucAppointment.NewCreatePrivateAppointment(
			appointmentRepo,
			auditDispatcher,
			resolveBookingPaymentPolicyUC,
		)

	completeAppointmentUC :=
		ucAppointment.NewCompleteAppointment(
			appointmentRepo,
			paymentRepo,
			auditDispatcher,
		)

	cancelAppointmentUC :=
		ucAppointment.NewCancelAppointment(
			appointmentRepo,
			paymentRepo,
			auditDispatcher,
		)

	listAppointmentsByDateUC :=
		ucAppointment.NewListAppointmentsByDate(
			appointmentRepo,
		)

	listAppointmentsByMonthUC :=
		ucAppointment.NewListAppointmentsByMonth(
			appointmentRepo,
		)

	// ======================================================
	// HANDLERS
	// ======================================================
	authHandler := handlers.NewAuthHandler(db, cfg)
	meHandler := handlers.NewMeHandler(db)
	barbershopHandler := handlers.NewBarbershopHandler(db)

	barberProductHandler := handlers.NewBarberProductHandler(db)
	clientHandler := handlers.NewClientHandler(db)
	workingHoursHandler := handlers.NewWorkingHoursHandler(db)

	appointmentHandler :=
		handlers.NewAppointmentHandler(
			createAppointmentUC,
			completeAppointmentUC,
			cancelAppointmentUC,
			listAppointmentsByDateUC,
			listAppointmentsByMonthUC,
		)

	paymentHandler :=
		handlers.NewPaymentHandler(
			listPaymentsUC,
		)

	paymentReportHandler :=
		handlers.NewPaymentReportHandler(
			getPaymentSummaryUC,
		)

	auditLogsHandler := handlers.NewAuditLogsHandler(db)

	publicHandler := handlers.NewPublicHandler(db)
	publicWebHandler := handlers.NewPublicWebHandler(db)
	appWebHandler := handlers.NewAppWebHandler(db)

	publicPaymentHandler :=
		handlers.NewPublicPaymentHandler(
			db,
			createPixPaymentUC,
		)

	pixWebhookHandler :=
		handlers.NewPixWebhookHandler(
			markPaymentAsPaidUC,
		)

	// ======================================================
	// WEB ROUTES
	// ======================================================
	r.GET("/web/public/:slug", publicWebHandler.ShowBookingPage)

	webApp := r.Group("/web/app")
	{
		webApp.GET("/login", appWebHandler.LoginPage)
		webApp.GET("/dashboard", appWebHandler.Dashboard)
		webApp.GET("/services", appWebHandler.Services)
	}

	// ======================================================
	// API ROUTES
	// ======================================================
	api := r.Group("/api")
	{
		// ------------------------------
		// PUBLIC API
		// ------------------------------
		publicAPI := api.Group("/public")
		{
			publicAPI.GET("/:slug/products", publicHandler.ListProducts)
			publicAPI.GET("/:slug/availability", publicHandler.AvailabilityForClient)
			publicAPI.POST("/:slug/appointments", publicHandler.CreateAppointment)

			publicAPI.POST(
				"/:slug/appointments/:id/payment/pix",
				publicPaymentHandler.CreatePix,
			)
		}

		// ------------------------------
		// PIX WEBHOOK
		// ------------------------------
		if cfg.PixWebhookSecret != "" {
			api.POST(
				"/webhooks/pix",
				middleware.NewPixWebhookAuth(cfg.PixWebhookSecret),
				pixWebhookHandler.Handle,
			)
		} else {
			api.POST(
				"/webhooks/pix",
				pixWebhookHandler.Handle,
			)
		}

		// ------------------------------
		// AUTH
		// ------------------------------
		api.POST("/auth/register", authHandler.Register)
		api.POST("/auth/login", authHandler.Login)

		// ------------------------------
		// SECURED API
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

			secured.POST("/me/appointments", appointmentHandler.Create)
			secured.GET("/me/appointments", appointmentHandler.ListByDate)
			secured.GET("/me/appointments/month", appointmentHandler.ListByMonth)
			secured.PATCH("/me/appointments/:id/cancel", appointmentHandler.Cancel)
			secured.PATCH("/me/appointments/:id/complete", appointmentHandler.Complete)

			secured.GET("/me/payments", paymentHandler.List)
			secured.GET("/me/payments/summary", paymentReportHandler.Summary)

			secured.GET("/me/audit-logs", auditLogsHandler.List)
		}
	}
}
