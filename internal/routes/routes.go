package routes

import (
	"context"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	"github.com/BruksfildServices01/barber-scheduler/internal/config"
	domainNotification "github.com/BruksfildServices01/barber-scheduler/internal/domain/notification"
	"github.com/BruksfildServices01/barber-scheduler/internal/handlers"
	cartStore "github.com/BruksfildServices01/barber-scheduler/internal/infra/cart"
	"github.com/BruksfildServices01/barber-scheduler/internal/infra/idempotency"
	"github.com/BruksfildServices01/barber-scheduler/internal/infra/notification"
	"github.com/BruksfildServices01/barber-scheduler/internal/infra/pix"
	infraRepo "github.com/BruksfildServices01/barber-scheduler/internal/infra/repository"
	"github.com/BruksfildServices01/barber-scheduler/internal/jobs"
	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	ucAppointment "github.com/BruksfildServices01/barber-scheduler/internal/usecase/appointment"
	ucCart "github.com/BruksfildServices01/barber-scheduler/internal/usecase/cart"
	ucMetrics "github.com/BruksfildServices01/barber-scheduler/internal/usecase/metrics"
	ucOrder "github.com/BruksfildServices01/barber-scheduler/internal/usecase/order"
	ucPayment "github.com/BruksfildServices01/barber-scheduler/internal/usecase/payment"
	paymentconfig "github.com/BruksfildServices01/barber-scheduler/internal/usecase/paymentconfig"
	ucProduct "github.com/BruksfildServices01/barber-scheduler/internal/usecase/product"
	ucPublic "github.com/BruksfildServices01/barber-scheduler/internal/usecase/public"
	ucService "github.com/BruksfildServices01/barber-scheduler/internal/usecase/service"
	ucTicket "github.com/BruksfildServices01/barber-scheduler/internal/usecase/ticket"
	ucServiceSuggestion "github.com/BruksfildServices01/barber-scheduler/internal/usecase/servicesuggestion"
	ucSubscription "github.com/BruksfildServices01/barber-scheduler/internal/usecase/subscription"

	"github.com/BruksfildServices01/barber-scheduler/internal/query/crm"
	"github.com/BruksfildServices01/barber-scheduler/internal/query/dashboard"
	"github.com/BruksfildServices01/barber-scheduler/internal/query/daypanel"
	"github.com/BruksfildServices01/barber-scheduler/internal/query/financial"
	"github.com/BruksfildServices01/barber-scheduler/internal/query/impact"
)

func RegisterRoutes(
	r *gin.Engine,
	db *gorm.DB,
	cfg *config.Config,
	scheduler *jobs.Scheduler,
) {
	// ======================================================
	// REPOSITORIES
	// ======================================================
	appointmentRepo := infraRepo.NewAppointmentGormRepository(db)
	paymentRepo := infraRepo.NewPaymentGormRepository(db)
	paymentConfigRepo := infraRepo.NewBarbershopPaymentConfigGormRepository(db)
	clientMetricsRepo := infraRepo.NewClientMetricsGormRepository(db)

	orderRepo := infraRepo.NewOrderGormRepository(db)
	productRepo := infraRepo.NewProductGormRepository(db)
	serviceRepo := infraRepo.NewServiceGormRepository(db)
	serviceSuggestionRepo := infraRepo.NewServiceSuggestionGormRepository(db)
	subscriptionRepo := infraRepo.NewSubscriptionGormRepository(db)

	ticketRepo := infraRepo.NewTicketGormRepository(db)

	idemStore := idempotency.NewGormStore(db)
	cartMemoryStore := cartStore.NewPostgresStore(db)

	// ======================================================
	// AUDIT
	// ======================================================
	auditLogger := audit.New(db)
	auditDispatcher := audit.NewDispatcher(auditLogger)

	// ======================================================
	// PIX
	// ======================================================
	// TODO: replace MockPixGateway with a real PIX integration (BB/Efí/etc)
	// before going live. The mock generates fake txid/qrcode for development only.
	pixGateway := pix.NewMockPixGateway()

	// ======================================================
	// RATE LIMITER
	// ======================================================
	// WARNING: this rate limiter is in-memory and per-instance.
	// In a multi-replica deployment each replica maintains its own bucket,
	// so the effective limit is (replicas × configured limit).
	// Replace with a Redis-backed limiter before scaling horizontally.
	log.Println("[WARN] rate limiter is in-memory — not suitable for multi-instance deployments")

	// ======================================================
	// NOTIFICATION
	// ======================================================
	var notifier domainNotification.Notifier
	if cfg.EmailEnabled {
		notifier = notification.NewEmailNotifier(cfg)
	} else {
		notifier = notification.NewNoopNotifier()
	}

	var apptNotifier domainNotification.AppointmentNotifier
	if cfg.EmailEnabled {
		apptNotifier = notification.NewAsyncAppointmentNotifier(notification.NewEmailNotifier(cfg))
	} else {
		apptNotifier = &notification.NoopAppointmentNotifier{}
	}

	// ======================================================
	// PAYMENT USE CASES
	// ======================================================
	createPixPaymentUC := ucPayment.NewCreatePixPayment(
		paymentRepo,
		pixGateway,
		auditDispatcher,
		idemStore,
	)

	createPixForOrderUC := ucPayment.NewCreatePixPaymentForOrder(
		paymentRepo,
		pixGateway,
		auditDispatcher,
	)

	markPaymentAsPaidUC := ucPayment.NewMarkPaymentAsPaid(
		paymentRepo,
		auditDispatcher,
		notifier,
		idemStore,
	)

	listPaymentsUC := ucPayment.NewListPaymentsForBarbershop(paymentRepo)
	getPaymentSummaryUC := ucPayment.NewGetPaymentSummary(paymentRepo)

	expirePaymentsUC := ucPayment.NewExpirePayments(
		paymentRepo,
		appointmentRepo,
		auditDispatcher,
	)

	createPaymentForAppointmentUC := ucPayment.NewCreatePaymentForAppointment(
		paymentRepo,
		appointmentRepo,
		paymentConfigRepo,
		auditDispatcher,
	)

	// ======================================================
	// METRICS USE CASES
	// ======================================================
	updateClientMetricsUC := ucMetrics.NewUpdateClientMetrics(clientMetricsRepo)
	getClientCategoryUC := ucMetrics.NewGetClientCategory(clientMetricsRepo)
	getClientsWithCategoryUC := ucMetrics.NewGetClientsWithCategory(clientMetricsRepo)
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
	// SERVICE USE CASES
	// ======================================================
	createServiceUC := ucService.NewCreateService(serviceRepo)
	updateServiceUC := ucService.NewUpdateService(serviceRepo)
	listPublicServicesUC := ucService.NewListPublicServices(serviceRepo)

	// ======================================================
	// SERVICE SUGGESTION USE CASES
	// ======================================================
	setServiceSuggestionUC := ucServiceSuggestion.NewSetServiceSuggestion(serviceSuggestionRepo)
	getServiceSuggestionUC := ucServiceSuggestion.NewGetServiceSuggestion(serviceSuggestionRepo)
	removeServiceSuggestionUC := ucServiceSuggestion.NewRemoveServiceSuggestion(serviceSuggestionRepo)
	getPublicServiceSuggestionUC := ucServiceSuggestion.NewGetPublicServiceSuggestion(serviceSuggestionRepo)

	// ======================================================
	// PRODUCT USE CASES
	// ======================================================
	createProductUC := ucProduct.NewCreateProduct(productRepo)
	updateProductUC := ucProduct.NewUpdateProduct(productRepo)
	listPublicProductsUC := ucProduct.NewListPublicProducts(productRepo)

	// ======================================================
	// ORDER USE CASES
	// ======================================================
	createOrderUC := ucOrder.NewCreateOrder(db, orderRepo, productRepo)
	getOrderUC := ucOrder.NewGetOrder(orderRepo)
	listOrdersAdminUC := ucOrder.NewListOrdersAdmin(orderRepo)

	// ======================================================
	// CART USE CASES
	// ======================================================
	getCartUC := ucCart.NewGetCart(cartMemoryStore)
	addCartItemUC := ucCart.NewAddItem(cartMemoryStore, productRepo)
	removeCartItemUC := ucCart.NewRemoveItem(cartMemoryStore)
	checkoutCartUC := ucCart.NewCheckoutCart(cartMemoryStore, createOrderUC)

	// ======================================================
	// APPOINTMENT USE CASES
	// ======================================================
	createAppointmentUC := ucAppointment.NewCreatePrivateAppointment(
		appointmentRepo,
		auditDispatcher,
		resolveBookingPaymentPolicyUC,
		updateClientMetricsUC,
		getClientCategoryUC,
		getActiveSubscriptionUC,
		idemStore,
	)

	completeAppointmentUC := ucAppointment.NewCompleteAppointment(
		db,
		appointmentRepo,
		paymentRepo,
		orderRepo,
		productRepo,
		auditDispatcher,
		updateClientMetricsUC,
		consumeCutUC,
	)

	cancelAppointmentUC := ucAppointment.NewCancelAppointment(
		appointmentRepo,
		auditDispatcher,
		updateClientMetricsUC,
	)

	markNoShowUC := ucAppointment.NewMarkAppointmentNoShow(
		appointmentRepo,
		auditDispatcher,
		updateClientMetricsUC,
	)

	listByDateUC := ucAppointment.NewListAppointmentsByDate(appointmentRepo)
	listByMonthUC := ucAppointment.NewListAppointmentsByMonth(appointmentRepo)
	createInternalAppointmentUC := ucAppointment.NewCreateInternalAppointment(appointmentRepo)
	getOperationalSummaryUC := ucAppointment.NewGetOperationalSummary(appointmentRepo)

	// ======================================================
	// TICKET USE CASES
	// ======================================================
	generateTicketUC := ucTicket.NewGenerateTicket(ticketRepo)
	viewTicketUC := ucTicket.NewViewTicket(db)
	cancelViaTicketUC := ucTicket.NewCancelViaTicket(db, ticketRepo, apptNotifier, updateClientMetricsUC, auditDispatcher)
	rescheduleViaTicketUC := ucTicket.NewRescheduleViaTicket(db, ticketRepo, apptNotifier, updateClientMetricsUC, auditDispatcher)

	// ======================================================
	// PUBLIC ORCHESTRATION USE CASES
	// ======================================================
	orchestratedCheckoutUC := ucPublic.NewOrchestratedCheckout(
		createAppointmentUC,
		getCartUC,
		checkoutCartUC,
		serviceRepo,
		generateTicketUC,
		db,
		apptNotifier,
	)

	// ======================================================
	// JOBS (P0.3 - leader lock Postgres)
	// ======================================================
	if scheduler != nil {
		locker := jobs.NewPostgresJobLocker(db, "")

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

		const every = time.Minute
		const ttl = 2 * time.Minute

		scheduler.Every(every, func(ctx context.Context) {
			ok, err := locker.TryLock(ctx, "job:expire_payments", ttl)
			if err != nil || !ok {
				return
			}
			expirePaymentsJob.Run(ctx)
			_ = locker.Unlock(ctx, "job:expire_payments")
		})

		scheduler.Every(every, func(ctx context.Context) {
			ok, err := locker.TryLock(ctx, "job:mark_no_show", ttl)
			if err != nil || !ok {
				return
			}
			if err := markNoShowJob.Run(ctx); err != nil {
				log.Printf("[MarkNoShowJob] error=%v\n", err)
			}
			_ = locker.Unlock(ctx, "job:mark_no_show")
		})
	}

	// ======================================================
	// HANDLERS
	// ======================================================
	authHandler := handlers.NewAuthHandler(db, cfg)
	meHandler := handlers.NewMeHandler(db)
	barbershopHandler := handlers.NewBarbershopHandler(db)

	serviceHandler := handlers.NewServiceHandler(
		db,
		createServiceUC,
		updateServiceUC,
	)

	serviceSuggestionHandler := handlers.NewServiceSuggestionHandler(
		setServiceSuggestionUC,
		getServiceSuggestionUC,
		removeServiceSuggestionUC,
	)

	productHandler := handlers.NewProductHandler(
		db,
		createProductUC,
		updateProductUC,
	)

	workingHoursHandler := handlers.NewWorkingHoursHandler(db, auditDispatcher)
	auditLogsHandler := handlers.NewAuditLogsHandler(db)

	clientHandler := handlers.NewClientHandler(
		db,
		getClientsWithCategoryUC,
		getActiveSubscriptionUC,
	)

	clientHistoryHandler := handlers.NewClientHistoryHandler(
		db,
		getClientCategoryUC,
		getActiveSubscriptionUC,
	)

	clientCategoryHandler := handlers.NewClientCategoryHandler(
		getClientCategoryUC,
		getActiveSubscriptionUC,
	)

	clientCategoryOverrideHandler := handlers.NewClientCategoryOverrideHandler(
		setClientCategoryUC,
	)

	paymentPolicyHandler := handlers.NewPaymentPolicyHandler(
		getPaymentPoliciesUC,
		updatePaymentPoliciesUC,
		auditDispatcher,
	)

	appointmentHandler := handlers.NewAppointmentHandler(
		createAppointmentUC,
		completeAppointmentUC,
		cancelAppointmentUC,
		markNoShowUC,
		listByDateUC,
		listByMonthUC,
	)

	internalAppointmentHandler := handlers.NewInternalAppointmentHandler(
		createInternalAppointmentUC,
	)

	publicHandler := handlers.NewPublicHandler(
		db,
		createAppointmentUC,
		listPublicServicesUC,
		listPublicProductsUC,
		getPublicServiceSuggestionUC,
		getCartUC,
		addCartItemUC,
		removeCartItemUC,
		checkoutCartUC,
	)

	publicCheckoutHandler := handlers.NewPublicCheckoutHandler(
		publicHandler,
		orchestratedCheckoutUC,
	)

	publicTicketHandler := handlers.NewPublicTicketHandler(viewTicketUC, cancelViaTicketUC, rescheduleViaTicketUC)

	publicPaymentHandler := handlers.NewPublicPaymentHandler(
		db,
		createPaymentForAppointmentUC,
		createPixPaymentUC,
	)

	pixWebhookHandler := handlers.NewPixWebhookHandler(markPaymentAsPaidUC)

	orderHandler := handlers.NewOrderHandler(
		createOrderUC,
		getOrderUC,
		listOrdersAdminUC,
	)
	orderPaymentHandler := handlers.NewOrderPaymentHandler(
		db,
		createPixForOrderUC,
		orderRepo,
	)

	paymentHandler := handlers.NewPaymentHandler(db, listPaymentsUC)
	paymentReportHandler := handlers.NewPaymentReportHandler(
		getPaymentSummaryUC,
		appointmentRepo,
	)
	operationalSummaryHandler := handlers.NewOperationalSummaryHandler(
		getOperationalSummaryUC,
	)
	planHandler := handlers.NewPlanHandler(createPlanUC, listPlansUC)

	dayPanelQuery := daypanel.New(db)
	dayPanelHandler := handlers.NewDayPanelHandler(dayPanelQuery)

	crmQuery := crm.New(db)
	crmHandler := handlers.NewCRMHandler(crmQuery)

	dashboardQuery := dashboard.New(db)
	dashboardHandler := handlers.NewDashboardHandler(dashboardQuery)

	financialQuery := financial.New(db)
	financialHandler := handlers.NewFinancialHandler(financialQuery)

	impactQuery := impact.New(db)
	impactHandler := handlers.NewImpactHandler(impactQuery)

	adjustClosureUC := ucAppointment.NewAdjustClosure(db, auditDispatcher)
	closureAdjustmentHandler := handlers.NewClosureAdjustmentHandler(adjustClosureUC)

	subscriptionHandler := handlers.NewSubscriptionHandler(
		activateSubscriptionUC,
		cancelSubscriptionUC,
		getActiveSubscriptionUC,
		auditDispatcher,
	)

	// ======================================================
	// ROUTES
	// ======================================================
	api := r.Group("/api")

	publicAPI := api.Group("/public")
	{
		publicAPI.GET("/:slug/services", publicHandler.ListServices)
		publicAPI.GET("/:slug/products", publicHandler.ListProducts)

		publicAPI.GET("/:slug/cart", publicHandler.GetCart)
		publicAPI.POST("/:slug/cart/items", publicHandler.AddCartItem)
		publicAPI.DELETE("/:slug/cart/items/:productId", publicHandler.RemoveCartItem)
		publicAPI.POST("/:slug/cart/checkout", publicHandler.CheckoutCart)
		publicAPI.POST("/:slug/checkout", publicCheckoutHandler.Checkout)

		publicAPI.GET("/:slug/services/:id/suggestion", publicHandler.GetServiceSuggestion)
		publicAPI.GET("/:slug/availability", publicHandler.AvailabilityForClient)
		publicAPI.POST("/:slug/appointments", publicHandler.CreateAppointment)

		publicAPI.POST(
			"/:slug/appointments/:id/payment/pix",
			middleware.RateLimitByKey(func(c *gin.Context) string {
				return middleware.ClientIPKey(c) + ":" + c.Param("slug")
			}, 30, 10),
			publicPaymentHandler.CreatePix,
		)
		publicAPI.POST(
			"/:slug/orders/:id/payment/pix",
			middleware.RateLimitByKey(func(c *gin.Context) string {
				return middleware.ClientIPKey(c) + ":" + c.Param("slug")
			}, 30, 10),
			orderPaymentHandler.CreatePublic,
		)

		publicAPI.GET("/ticket/:token", publicTicketHandler.View)
		publicAPI.DELETE("/ticket/:token", publicTicketHandler.Cancel)
		publicAPI.PATCH("/ticket/:token", publicTicketHandler.Reschedule)
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

		secured.GET("/me/services", serviceHandler.List)
		secured.POST("/me/services", serviceHandler.Create)
		secured.PUT("/me/services/:id", serviceHandler.Update)

		secured.GET("/me/services/:id/suggestion", serviceSuggestionHandler.Get)
		secured.PUT("/me/services/:id/suggestion", serviceSuggestionHandler.Set)
		secured.DELETE("/me/services/:id/suggestion", serviceSuggestionHandler.Remove)

		secured.GET("/me/products", productHandler.List)
		secured.POST("/me/products", productHandler.Create)
		secured.PUT("/me/products/:id", productHandler.Update)

		secured.GET("/me/working-hours", workingHoursHandler.Get)
		secured.PUT("/me/working-hours", workingHoursHandler.Update)

		secured.GET("/me/clients", clientHandler.List)
		secured.GET("/me/clients/:id/crm", crmHandler.Get)
		secured.GET("/me/clients/:id/history", clientHistoryHandler.Get)
		secured.GET("/me/clients/:id/category", clientCategoryHandler.Get)
		secured.PUT("/me/clients/:id/category", clientCategoryOverrideHandler.Update)

		secured.GET("/me/payment-policies", paymentPolicyHandler.Get)
		secured.PUT("/me/payment-policies", paymentPolicyHandler.Update)

		secured.POST("/me/appointments", appointmentHandler.Create)
		secured.PUT("/me/appointments/:id/complete", appointmentHandler.Complete)
		secured.PUT("/me/appointments/:id/cancel", appointmentHandler.Cancel)
		secured.PUT("/me/appointments/:id/no-show", appointmentHandler.MarkNoShow)
		secured.POST("/me/appointments/:id/closure/adjustment", closureAdjustmentHandler.Create)
		secured.GET("/me/appointments/date", appointmentHandler.ListByDate)
		secured.GET("/me/appointments/month", appointmentHandler.ListByMonth)

		secured.POST("/me/internal-appointments", internalAppointmentHandler.Create)

		secured.GET("/me/payments", paymentHandler.List)
		secured.GET("/me/summary", operationalSummaryHandler.Get)
		secured.GET("/me/payments/summary", paymentReportHandler.Summary)

		secured.POST("/me/orders", orderHandler.Create)
		secured.GET("/me/orders", orderHandler.List)
		secured.GET("/me/orders/:id", orderHandler.GetByID)
		secured.POST("/me/orders/:id/payment/pix", orderPaymentHandler.Create)

		secured.GET("/me/audit-logs", auditLogsHandler.List)

		secured.POST("/me/plans", planHandler.Create)
		secured.GET("/me/plans", planHandler.List)

		secured.GET("/me/dashboard", dashboardHandler.Get)
		secured.GET("/me/financial", financialHandler.Get)
		secured.GET("/me/day-panel", dayPanelHandler.Get)
		secured.GET("/me/impact", impactHandler.Get)

		secured.POST("/me/subscriptions", subscriptionHandler.Activate)
		secured.DELETE("/me/subscriptions/:clientID", subscriptionHandler.Cancel)
		secured.GET("/me/subscriptions/:clientID", subscriptionHandler.GetActive)
	}
}
