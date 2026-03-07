package routes

import (
	"context"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	"github.com/BruksfildServices01/barber-scheduler/internal/config"
	"github.com/BruksfildServices01/barber-scheduler/internal/handlers"
	"github.com/BruksfildServices01/barber-scheduler/internal/infra/idempotency"
	"github.com/BruksfildServices01/barber-scheduler/internal/infra/notification"
	"github.com/BruksfildServices01/barber-scheduler/internal/infra/pix"
	infraRepo "github.com/BruksfildServices01/barber-scheduler/internal/infra/repository"
	"github.com/BruksfildServices01/barber-scheduler/internal/jobs"
	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"

	domainNotification "github.com/BruksfildServices01/barber-scheduler/internal/domain/notification"

	ucAppointment "github.com/BruksfildServices01/barber-scheduler/internal/usecase/appointment"
	ucMetrics "github.com/BruksfildServices01/barber-scheduler/internal/usecase/metrics"
	ucOrder "github.com/BruksfildServices01/barber-scheduler/internal/usecase/order"
	ucPayment "github.com/BruksfildServices01/barber-scheduler/internal/usecase/payment"
	paymentconfig "github.com/BruksfildServices01/barber-scheduler/internal/usecase/paymentconfig"
	ucSubscription "github.com/BruksfildServices01/barber-scheduler/internal/usecase/subscription"
)

func RegisterRoutes(r *gin.Engine, db *gorm.DB, cfg *config.Config, scheduler *jobs.Scheduler) {

	// ======================================================
	// REPOSITORIES
	// ======================================================
	appointmentRepo := infraRepo.NewAppointmentGormRepository(db)
	paymentRepo := infraRepo.NewPaymentGormRepository(db)
	paymentConfigRepo := infraRepo.NewBarbershopPaymentConfigGormRepository(db)
	clientMetricsRepo := infraRepo.NewClientMetricsGormRepository(db)

	orderRepo := infraRepo.NewOrderGormRepository(db)
	productRepo := infraRepo.NewProductGormRepository(db) // ✅ NOVO (Order precisa)
	subscriptionRepo := infraRepo.NewSubscriptionGormRepository(db)

	idemStore := idempotency.NewGormStore(db)

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
	// PAYMENT USE CASES
	// ======================================================
	createPixPaymentUC :=
		ucPayment.NewCreatePixPayment(
			paymentRepo,
			pixGateway,
			auditDispatcher,
			idemStore,
		)

	createPixForOrderUC :=
		ucPayment.NewCreatePixPaymentForOrder(
			paymentRepo,
			pixGateway,
			auditDispatcher,
		)

	markPaymentAsPaidUC :=
		ucPayment.NewMarkPaymentAsPaid(
			paymentRepo,
			auditDispatcher,
			notifier,
			idemStore,
		)

	listPaymentsUC := ucPayment.NewListPaymentsForBarbershop(paymentRepo)
	getPaymentSummaryUC := ucPayment.NewGetPaymentSummary(paymentRepo)

	expirePaymentsUC :=
		ucPayment.NewExpirePayments(
			paymentRepo,
			appointmentRepo,
			auditDispatcher,
		)

	// ======================================================
	// METRICS USE CASES
	// ======================================================
	updateClientMetricsUC := ucMetrics.NewUpdateClientMetrics(clientMetricsRepo)
	getClientCategoryUC := ucMetrics.NewGetClientCategory(clientMetricsRepo, subscriptionRepo)
	getClientsWithCategoryUC := ucMetrics.NewGetClientsWithCategory(clientMetricsRepo, subscriptionRepo)
	setClientCategoryUC := ucMetrics.NewSetClientCategory(clientMetricsRepo)

	// ======================================================
	// SUBSCRIPTION USE CASES
	// ======================================================
	consumeCutUC := ucSubscription.NewConsumeCut(subscriptionRepo)
	createPlanUC := ucSubscription.NewCreatePlan(subscriptionRepo)
	listPlansUC := ucSubscription.NewListPlans(subscriptionRepo)
	activateSubscriptionUC := ucSubscription.NewActivateSubscription(subscriptionRepo)
	cancelSubscriptionUC := ucSubscription.NewCancelSubscription(subscriptionRepo)
	getActiveSubscriptionUC := ucSubscription.NewGetActiveSubscription(subscriptionRepo)

	// ======================================================
	// PAYMENT CONFIG
	// ======================================================
	getPaymentPoliciesUC := paymentconfig.NewGetPaymentPolicies(paymentConfigRepo)
	updatePaymentPoliciesUC := paymentconfig.NewUpdatePaymentPolicies(paymentConfigRepo)
	resolveBookingPaymentPolicyUC := paymentconfig.NewResolveBookingPaymentPolicy(paymentConfigRepo)

	// ======================================================
	// ORDER USE CASES (✅ NOVO)
	// ======================================================
	createOrderUC := ucOrder.NewCreateOrder(db, orderRepo, productRepo)

	// ======================================================
	// APPOINTMENT USE CASES
	// ======================================================
	createAppointmentUC :=
		ucAppointment.NewCreatePrivateAppointment(
			appointmentRepo,
			auditDispatcher,
			resolveBookingPaymentPolicyUC,
			updateClientMetricsUC,
			getClientCategoryUC,
			getActiveSubscriptionUC,
			idemStore,
		)

	createPaymentForAppointmentUC :=
		ucPayment.NewCreatePaymentForAppointment(
			paymentRepo,
			appointmentRepo,
			paymentConfigRepo,
			auditDispatcher,
		)

	completeAppointmentUC :=
		ucAppointment.NewCompleteAppointment(
			db,
			appointmentRepo,
			paymentRepo,
			auditDispatcher,
			updateClientMetricsUC,
			consumeCutUC,
		)

	cancelAppointmentUC :=
		ucAppointment.NewCancelAppointment(
			appointmentRepo,
			auditDispatcher,
			updateClientMetricsUC,
		)

	markNoShowUC :=
		ucAppointment.NewMarkAppointmentNoShow(
			appointmentRepo,
			auditDispatcher,
			updateClientMetricsUC,
		)

	listByDateUC := ucAppointment.NewListAppointmentsByDate(appointmentRepo)
	listByMonthUC := ucAppointment.NewListAppointmentsByMonth(appointmentRepo)
	createInternalAppointmentUC := ucAppointment.NewCreateInternalAppointment(appointmentRepo)
	getOperationalSummaryUC := ucAppointment.NewGetOperationalSummary(appointmentRepo)

	// ======================================================
	// JOBS (P0.3 - leader lock Postgres)
	// ======================================================
	if scheduler != nil {

		locker := jobs.NewPostgresJobLocker(db, "") // owner auto (hostname:pid)

		expirePaymentsJob := jobs.NewExpirePaymentsJob(
			expirePaymentsUC,
			appointmentRepo,
		)

		markNoShowJob := jobs.NewMarkNoShowJob(
			appointmentRepo,
			updateClientMetricsUC,
			auditDispatcher,
			appointmentRepo,
		)

		// TTL > intervalo (evita dois nós rodarem se o job atrasar um pouco)
		const every = time.Minute
		const ttl = 2 * time.Minute

		scheduler.Every(every, func(ctx context.Context) {
			ok, err := locker.TryLock(ctx, "job:expire_payments", ttl)
			if err != nil || !ok {
				return
			}
			expirePaymentsJob.Run(ctx)
			_ = locker.Unlock(ctx, "job:expire_payments") // best-effort
		})

		scheduler.Every(every, func(ctx context.Context) {
			ok, err := locker.TryLock(ctx, "job:mark_no_show", ttl)
			if err != nil || !ok {
				return
			}
			if err := markNoShowJob.Run(ctx); err != nil {
				log.Printf("[MarkNoShowJob] error=%v\n", err)
			}
			_ = locker.Unlock(ctx, "job:mark_no_show") // best-effort
		})
	}

	// ======================================================
	// HANDLERS
	// ======================================================
	authHandler := handlers.NewAuthHandler(db, cfg)
	meHandler := handlers.NewMeHandler(db)
	barbershopHandler := handlers.NewBarbershopHandler(db)
	barberProductHandler := handlers.NewBarberProductHandler(db)
	workingHoursHandler := handlers.NewWorkingHoursHandler(db)
	auditLogsHandler := handlers.NewAuditLogsHandler(db)

	clientHandler := handlers.NewClientHandler(db, getClientsWithCategoryUC)
	clientHistoryHandler := handlers.NewClientHistoryHandler(db, getClientCategoryUC)
	clientCategoryHandler := handlers.NewClientCategoryHandler(getClientCategoryUC)
	clientCategoryOverrideHandler := handlers.NewClientCategoryOverrideHandler(setClientCategoryUC)

	paymentPolicyHandler := handlers.NewPaymentPolicyHandler(getPaymentPoliciesUC, updatePaymentPoliciesUC)

	appointmentHandler := handlers.NewAppointmentHandler(
		createAppointmentUC,
		completeAppointmentUC,
		cancelAppointmentUC,
		markNoShowUC,
		listByDateUC,
		listByMonthUC,
	)

	internalAppointmentHandler := handlers.NewInternalAppointmentHandler(createInternalAppointmentUC)
	publicHandler := handlers.NewPublicHandler(db, createAppointmentUC)

	publicPaymentHandler := handlers.NewPublicPaymentHandler(
		db,
		createPaymentForAppointmentUC,
		createPixPaymentUC,
	)

	pixWebhookHandler := handlers.NewPixWebhookHandler(markPaymentAsPaidUC)

	// ✅ Order handlers
	orderHandler := handlers.NewOrderHandler(createOrderUC) // ✅ NOVO
	orderPaymentHandler := handlers.NewOrderPaymentHandler(createPixForOrderUC, orderRepo)

	paymentHandler := handlers.NewPaymentHandler(db, listPaymentsUC)
	paymentReportHandler := handlers.NewPaymentReportHandler(getPaymentSummaryUC, appointmentRepo)
	operationalSummaryHandler := handlers.NewOperationalSummaryHandler(getOperationalSummaryUC)
	planHandler := handlers.NewPlanHandler(createPlanUC, listPlansUC)

	subscriptionHandler := handlers.NewSubscriptionHandler(
		activateSubscriptionUC,
		cancelSubscriptionUC,
		getActiveSubscriptionUC,
	)

	// ======================================================
	// ROUTES
	// ======================================================
	api := r.Group("/api")

	publicAPI := api.Group("/public")
	{
		publicAPI.GET("/:slug/products", publicHandler.ListProducts)
		publicAPI.GET("/:slug/availability", publicHandler.AvailabilityForClient)
		publicAPI.POST("/:slug/appointments", publicHandler.CreateAppointment)

		publicAPI.POST(
			"/:slug/appointments/:id/payment/pix",
			middleware.RateLimitByKey(func(c *gin.Context) string {
				return middleware.ClientIPKey(c) + ":" + c.Param("slug")
			}, 30, 10),
			publicPaymentHandler.CreatePix,
		)
	}

	api.POST(
		"/webhooks/pix",
		middleware.MaxBodySize(64*1024),
		middleware.NewPixWebhookAuth(cfg.PixWebhookSecret),
		pixWebhookHandler.Handle,
	)

	api.POST("/auth/register", authHandler.Register)
	api.POST("/auth/login", authHandler.Login)

	secured := api.Group("/")
	secured.Use(middleware.AuthMiddleware(cfg))
	{
		secured.GET("/me", meHandler.GetMe)
		secured.GET("/me/barbershop", barbershopHandler.GetMeBarbershop)
		secured.PUT("/me/barbershop", barbershopHandler.UpdateMeBarbershop)

		secured.GET("/me/services", barberProductHandler.List)
		secured.POST("/me/services", barberProductHandler.Create)
		secured.PUT("/me/services/:id", barberProductHandler.Update)

		secured.GET("/me/working-hours", workingHoursHandler.Get)
		secured.PUT("/me/working-hours", workingHoursHandler.Update)

		secured.GET("/me/clients", clientHandler.List)
		secured.GET("/me/clients/:id/history", clientHistoryHandler.Get)
		secured.GET("/me/clients/:id/category", clientCategoryHandler.Get)
		secured.PUT("/me/clients/:id/category", clientCategoryOverrideHandler.Update)

		secured.GET("/me/payment-policies", paymentPolicyHandler.Get)
		secured.PUT("/me/payment-policies", paymentPolicyHandler.Update)

		secured.POST("/me/appointments", appointmentHandler.Create)
		secured.PUT("/me/appointments/:id/complete", appointmentHandler.Complete)
		secured.PUT("/me/appointments/:id/cancel", appointmentHandler.Cancel)
		secured.PUT("/me/appointments/:id/no-show", appointmentHandler.MarkNoShow)
		secured.GET("/me/appointments/date", appointmentHandler.ListByDate)
		secured.GET("/me/appointments/month", appointmentHandler.ListByMonth)

		secured.POST("/me/internal-appointments", internalAppointmentHandler.Create)

		secured.GET("/me/payments", paymentHandler.List)
		secured.GET("/me/summary", operationalSummaryHandler.Get)
		secured.GET("/me/payments/summary", paymentReportHandler.Summary)

		// ✅ Orders
		secured.POST("/me/orders", orderHandler.Create) // ✅ NOVO
		secured.POST("/me/orders/:id/payment/pix", orderPaymentHandler.Create)

		secured.GET("/me/audit-logs", auditLogsHandler.List)

		secured.POST("/me/plans", planHandler.Create)
		secured.GET("/me/plans", planHandler.List)

		secured.POST("/me/subscriptions", subscriptionHandler.Activate)
		secured.DELETE("/me/subscriptions/:clientID", subscriptionHandler.Cancel)
		secured.GET("/me/subscriptions/:clientID", subscriptionHandler.GetActive)
	}
}
