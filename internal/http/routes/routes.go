package routes

import (
	"context"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	"github.com/BruksfildServices01/barber-scheduler/internal/config"
	domainPayment "github.com/BruksfildServices01/barber-scheduler/internal/domain/payment"
	domainNotification "github.com/BruksfildServices01/barber-scheduler/internal/domain/notification"
	"github.com/BruksfildServices01/barber-scheduler/internal/http/handlers"
	cartStore "github.com/BruksfildServices01/barber-scheduler/internal/cart"
	"github.com/BruksfildServices01/barber-scheduler/internal/security/crypt"
	"github.com/BruksfildServices01/barber-scheduler/internal/idempotency"
	"github.com/BruksfildServices01/barber-scheduler/internal/notification"
	"github.com/BruksfildServices01/barber-scheduler/internal/integration/payment/mercadopago"
	paymentinfra "github.com/BruksfildServices01/barber-scheduler/internal/integration/payment"
	gcal "github.com/BruksfildServices01/barber-scheduler/internal/integration/calendar"
	infraRepo "github.com/BruksfildServices01/barber-scheduler/internal/repository"
	"github.com/BruksfildServices01/barber-scheduler/internal/storage"
	"github.com/BruksfildServices01/barber-scheduler/internal/jobs"
	"github.com/BruksfildServices01/barber-scheduler/internal/http/middleware"
	ucAppointment "github.com/BruksfildServices01/barber-scheduler/internal/usecase/appointment"
	ucCart        "github.com/BruksfildServices01/barber-scheduler/internal/usecase/cart"
	ucClientPkg   "github.com/BruksfildServices01/barber-scheduler/internal/usecase/client"
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
	subscription "github.com/BruksfildServices01/barber-scheduler/internal/query/subscription"
)

// RegisterRoutes configura todas as rotas e retorna o Dispatcher de auditoria
// para que o main possa chamar Shutdown() no graceful shutdown.
func RegisterRoutes(
	r *gin.Engine,
	db *gorm.DB,
	cfg *config.Config,
	scheduler *jobs.Scheduler,
) *audit.Dispatcher {
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
	// MERCADO PAGO
	// ======================================================
	// Seleciona o gateway via MP_PROVIDER:
	//   "mp"   → integração real Mercado Pago (requer MP_ACCESS_TOKEN)
	//   "mock" → gateway falso para desenvolvimento (padrão)
	var mpGateway domainPayment.MPGateway
	var transparentGateway domainPayment.TransparentGateway
	if cfg.MPProvider == "mp" && cfg.MPAccessToken != "" {
		gw, err := mp.New(cfg.MPAccessToken)
		if err != nil {
			log.Fatal("[MP] falha ao inicializar gateway:", err)
		}
		mpGateway = gw
		transparentGateway = gw
		log.Println("[MP] usando gateway Mercado Pago real")
	} else {
		mock := mp.NewMockGateway()
		mpGateway = mock
		transparentGateway = mock
		log.Println("[MP] usando MockGateway — NÃO use em produção")
	}

	// ======================================================
	// RATE LIMITER
	// ======================================================
	// Usa Redis (distribuído) se REDIS_URL estiver configurada;
	// caso contrário usa in-memory por instância (desenvolvimento).

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
		apptNotifier = notification.NewNoopNotifier()
	}

	// ======================================================
	// PAYMENT USE CASES
	// ======================================================
	createMPPreferenceUC := ucPayment.NewCreateMPPreference(
		paymentRepo,
		mpGateway,
		auditDispatcher,
		cfg.AppURL,
		cfg.BackendURL,
	)

	createTransparentPaymentUC := ucPayment.NewCreateTransparentPayment(
		paymentRepo,
		transparentGateway,
		auditDispatcher,
		cfg.BackendURL,
		db,
		apptNotifier,
		ticketRepo,
		cfg.AppURL,
	)

	markMPPaymentAsPaidUC := ucPayment.NewMarkMPPaymentAsPaid(
		paymentRepo,
		auditDispatcher,
		notifier,
		idemStore,
		db,
		apptNotifier,
		ticketRepo,
		cfg.AppURL,
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
	updateClientMetricsUC := ucMetrics.NewUpdateClientMetrics(clientMetricsRepo, db)
	getClientCategoryUC := ucMetrics.NewGetClientCategory(clientMetricsRepo)
	getClientsWithCategoryUC := ucMetrics.NewGetClientsWithCategory(clientMetricsRepo)
	setClientCategoryUC := ucMetrics.NewSetClientCategory(clientMetricsRepo)

	// ======================================================
	// SUBSCRIPTION USE CASES
	// ======================================================
	consumeCutUC := ucSubscription.NewConsumeCut(subscriptionRepo)
	reserveSubscriptionCutUC := ucSubscription.NewReserveSubscriptionCut(subscriptionRepo)
	releaseSubscriptionCutUC := ucSubscription.NewReleaseSubscriptionCut(subscriptionRepo)
	createPlanUC := ucSubscription.NewCreatePlan(subscriptionRepo)
	updatePlanUC := ucSubscription.NewUpdatePlan(subscriptionRepo)
	setPlanActiveUC := ucSubscription.NewSetPlanActive(subscriptionRepo)
	listPlansUC := ucSubscription.NewListPlans(subscriptionRepo)
	deletePlanUC := ucSubscription.NewDeletePlan(subscriptionRepo)
	activateSubscriptionUC := ucSubscription.NewActivateSubscription(subscriptionRepo)
	cancelSubscriptionUC := ucSubscription.NewCancelSubscription(subscriptionRepo)
	getActiveSubscriptionUC := ucSubscription.NewGetActiveSubscription(subscriptionRepo)
	purchaseSubscriptionUC := ucSubscription.NewPurchaseSubscription(
		subscriptionRepo,
		paymentRepo,
		transparentGateway,
		auditDispatcher,
		db,
		cfg.BackendURL,
	)

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
	checkoutCartUC := ucCart.NewCheckoutCart(db, cartMemoryStore, createOrderUC)

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
		reserveSubscriptionCutUC,
		idemStore,
	)

	completeAppointmentUC := ucAppointment.NewCompleteAppointment(
		db,
		appointmentRepo,
		paymentRepo,
		orderRepo,
		productRepo,
		subscriptionRepo,
		auditDispatcher,
		updateClientMetricsUC,
		consumeCutUC,
	)

	cancelAppointmentUC := ucAppointment.NewCancelAppointment(
		db,
		appointmentRepo,
		subscriptionRepo,
		auditDispatcher,
		updateClientMetricsUC,
		releaseSubscriptionCutUC,
	)

	markNoShowUC := ucAppointment.NewMarkAppointmentNoShow(
		db,
		appointmentRepo,
		subscriptionRepo,
		auditDispatcher,
		updateClientMetricsUC,
		releaseSubscriptionCutUC,
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
	rescheduleViaTicketUC := ucTicket.NewRescheduleViaTicket(db, ticketRepo, apptNotifier, updateClientMetricsUC, auditDispatcher, cfg.AppURL)

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
		cfg.AppURL,
		getPublicServiceSuggestionUC,
	)

	// ======================================================
	// JOBS (P0.3 - leader lock Postgres)
	// ======================================================
	if scheduler != nil {
		locker := jobs.NewPostgresJobLocker(db, "")

		expirePaymentsJob := jobs.NewExpirePaymentsJob(
			expirePaymentsUC,
			appointmentRepo,
			appointmentRepo,
		)

		autoCompleteJob := jobs.NewAutoCompleteJob(
			completeAppointmentUC,
			appointmentRepo,
			auditDispatcher,
			appointmentRepo,
		)

		expireSubscriptionsUC := ucSubscription.NewExpireSubscriptions(subscriptionRepo)
		expireSubscriptionsJob := jobs.NewExpireSubscriptionsJob(expireSubscriptionsUC)

		const everyExpire = 10 * time.Minute
		const ttlExpire = 13 * time.Minute
		const everyAutoComplete = 50 * time.Minute
		const ttlAutoComplete = 55 * time.Minute

		scheduler.Every(everyExpire, func(ctx context.Context) {
			ok, err := locker.TryLock(ctx, "job:expire_payments", ttlExpire)
			if err != nil || !ok {
				return
			}
			expirePaymentsJob.Run(ctx)
			_ = locker.Unlock(ctx, "job:expire_payments")
		})

		scheduler.Every(everyAutoComplete, func(ctx context.Context) {
			ok, err := locker.TryLock(ctx, "job:auto_complete", ttlAutoComplete)
			if err != nil || !ok {
				return
			}
			if err := autoCompleteJob.Run(ctx); err != nil {
				log.Printf("[AutoCompleteJob] error=%v\n", err)
			}
			_ = locker.Unlock(ctx, "job:auto_complete")
		})

		const everyHour = time.Hour
		const ttlHour = 90 * time.Minute

		scheduler.Every(everyHour, func(ctx context.Context) {
			ok, err := locker.TryLock(ctx, "job:expire_subscriptions", ttlHour)
			if err != nil || !ok {
				return
			}
			expireSubscriptionsJob.Run(ctx)
			_ = locker.Unlock(ctx, "job:expire_subscriptions")
		})

		pruneJob := jobs.NewPruneJob(db)
		const everyDay = 24 * time.Hour
		const ttlDay = 25 * time.Hour

		scheduler.Every(everyDay, func(ctx context.Context) {
			ok, err := locker.TryLock(ctx, "job:prune", ttlDay)
			if err != nil || !ok {
				return
			}
			pruneJob.Run(ctx)
			_ = locker.Unlock(ctx, "job:prune")
		})
	}

	// ======================================================
	// HANDLERS
	// ======================================================
	authHandler := handlers.NewAuthHandler(db, cfg)

	var pwMailer handlers.PasswordMailer
	if cfg.EmailEnabled {
		pwMailer = notification.NewEmailNotifier(cfg)
	} else {
		pwMailer = notification.NewNoopNotifier()
	}
	passwordResetHandler := handlers.NewPasswordResetHandler(db, cfg.AppURL, pwMailer)
	meHandler := handlers.NewMeHandler(db)
	barbershopHandler := handlers.NewBarbershopHandler(db)

	serviceHandler := handlers.NewServiceHandler(
		db,
		createServiceUC,
		updateServiceUC,
	)

	serviceCategoryHandler := handlers.NewServiceCategoryHandler(db)

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
	scheduleOverrideHandler := handlers.NewScheduleOverrideHandler(db)
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
		auditDispatcher,
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

	whatsappHandler        := handlers.NewWhatsAppHandler(db, cfg.EvolutionURL, cfg.EvolutionAPIKey, cfg.BackendURL)
	whatsappWebhookHandler := handlers.NewWhatsAppWebhookHandler(db, cfg.EvolutionURL, cfg.EvolutionAPIKey)

	mpOAuthHandler := handlers.NewMPOAuthHandler(
		db,
		cfg.MPClientID,
		cfg.MPClientSecret,
		cfg.BackendURL+"/api/mercadopago/oauth/callback",
		cfg.AppURL,
		cfg.JWTSecret,
	)

	googleCalCfg := gcal.OAuthConfig{
		ClientID:     cfg.GoogleClientID,
		ClientSecret: cfg.GoogleClientSecret,
		RedirectURL:  cfg.GoogleRedirectURL,
	}

	appointmentHandler := handlers.NewAppointmentHandler(
		createAppointmentUC,
		completeAppointmentUC,
		cancelAppointmentUC,
		markNoShowUC,
		listByDateUC,
		listByMonthUC,
		db,
		googleCalCfg,
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

	mpPaymentHandler := handlers.NewMPPaymentHandler(
		db,
		createPaymentForAppointmentUC,
		createMPPreferenceUC,
	)

	var paymentCipher *crypt.Cipher
	if cfg.PaymentCredentialsEncryptionKey != "" {
		c, err := crypt.NewCipher(cfg.PaymentCredentialsEncryptionKey)
		if err != nil {
			log.Fatalf("[PAYMENT] chave de criptografia inválida: %v", err)
		}
		paymentCipher = c
		log.Println("[PAYMENT] cipher inicializado para credentials_encrypted")
	}

	providerRegistry := paymentinfra.NewProviderRegistry(db, paymentCipher, cfg.PagBankSandbox)

	pagbankOAuthHandler := handlers.NewPagBankOAuthHandler(
		db,
		cfg.PagBankClientID,
		cfg.PagBankClientSecret,
		cfg.BackendURL+"/api/pagbank/oauth/callback",
		cfg.AppURL,
		cfg.JWTSecret,
		paymentCipher,
		cfg.PagBankSandbox,
	)

	pagbankWebhookHandler := handlers.NewPagBankWebhookHandler(
		db,
		markMPPaymentAsPaidUC,
		paymentCipher,
		cfg.PagBankSandbox,
	)

	transparentPaymentHandler := handlers.NewTransparentPaymentHandler(
		db,
		createPaymentForAppointmentUC,
		createTransparentPaymentUC,
		providerRegistry,
	)

	mpWebhookHandler := handlers.NewMPWebhookHandler(
		markMPPaymentAsPaidUC,
		cfg.MPAccessToken,
		cfg.MPWebhookSecret,
		cfg.MPProvider == "mp", // requireSignature: obrigatório quando em modo produção real
		db,
	).WithRegistry(providerRegistry)

	orderHandler := handlers.NewOrderHandler(
		createOrderUC,
		getOrderUC,
		listOrdersAdminUC,
		orderRepo,
	)

	closureListHandler := handlers.NewClosureListHandler(db)

	paymentHandler := handlers.NewPaymentHandler(db, listPaymentsUC)
	paymentReportHandler := handlers.NewPaymentReportHandler(
		getPaymentSummaryUC,
		appointmentRepo,
	)
	operationalSummaryHandler := handlers.NewOperationalSummaryHandler(
		getOperationalSummaryUC,
	)
	planHandler := handlers.NewPlanHandler(createPlanUC, updatePlanUC, setPlanActiveUC, listPlansUC, deletePlanUC)

	dayPanelQuery := daypanel.New(db)
	dayPanelHandler := handlers.NewDayPanelHandler(dayPanelQuery)

	crmQuery := crm.New(db)
	crmHandler := handlers.NewCRMHandler(crmQuery, auditDispatcher)

	anonymizeClientUC      := ucClientPkg.NewAnonymizeClient(db, auditDispatcher)
	clientAnonymizeHandler := handlers.NewClientAnonymizeHandler(anonymizeClientUC)

	dashboardQuery := dashboard.New(db)
	dashboardHandler := handlers.NewDashboardHandler(dashboardQuery)

	financialQuery := financial.New(db)
	financialHandler := handlers.NewFinancialHandler(financialQuery)

	impactQuery := impact.New(db)
	impactHandler := handlers.NewImpactHandler(impactQuery)

	adjustClosureUC := ucAppointment.NewAdjustClosure(db, auditDispatcher)
	closureAdjustmentHandler := handlers.NewClosureAdjustmentHandler(adjustClosureUC)

	// R2 storage — only active when credentials are configured.
	var imageHandler *handlers.ImageHandler
	if cfg.R2AccountID != "" && cfg.R2BucketName != "" {
		r2 := storage.NewR2Service(
			cfg.R2AccountID,
			cfg.R2AccessKeyID,
			cfg.R2SecretAccessKey,
			cfg.R2BucketName,
			cfg.R2PublicURL,
		)
		imageHandler = handlers.NewImageHandler(db, r2)
		log.Println("[R2] storage enabled, bucket:", cfg.R2BucketName)
	} else {
		log.Println("[R2] storage disabled (credentials not set)")
	}

	subscriptionQuery := subscription.New(db)

	subscriptionHandler := handlers.NewSubscriptionHandler(
		activateSubscriptionUC,
		cancelSubscriptionUC,
		getActiveSubscriptionUC,
		subscriptionQuery,
		auditDispatcher,
	)

	publicSubscriptionHandler := handlers.NewPublicSubscriptionHandler(
		db,
		listPlansUC,
		purchaseSubscriptionUC,
		providerRegistry,
	)

	billingHandler := handlers.NewBillingHandler(db, cfg, idemStore)

	// ======================================================
	// ROUTES
	// ======================================================
	api := r.Group("/api")

	registerPublicRoutes(api, cfg, publicHandler, publicCheckoutHandler,
		mpPaymentHandler, transparentPaymentHandler,
		publicSubscriptionHandler, publicTicketHandler)

	// Fallback para quando o webhook MP não chega: frontend consulta status diretamente.
	api.GET("/public/:slug/appointments/:id/payment/status",
		middleware.NewRateLimitByKey(func(c *gin.Context) string {
			return middleware.ClientIPKey(c) + ":" + c.Param("slug")
		}, 10, 60, cfg.RedisURL),
		mpWebhookHandler.CheckPaymentStatus,
	)

	registerWebhookAndAuthRoutes(r, api, cfg, mpWebhookHandler, billingHandler,
		authHandler, passwordResetHandler)

	secured := api.Group("/")
	secured.Use(middleware.AuthMiddleware(cfg, db))

	registerCatalogRoutes(secured, meHandler, barbershopHandler,
		serviceHandler, serviceCategoryHandler, serviceSuggestionHandler,
		productHandler, workingHoursHandler, scheduleOverrideHandler)

	registerClientRoutes(secured, clientHandler, clientHistoryHandler,
		clientCategoryHandler, clientCategoryOverrideHandler, crmHandler,
		clientAnonymizeHandler, paymentPolicyHandler)

	// ── WhatsApp ────────────────────────────────────────────────────────────
	// ── Mercado Pago OAuth ─────────────────────────────────────────
	secured.GET("/me/mercadopago/oauth/start",    mpOAuthHandler.Start)
	secured.GET("/me/mercadopago/oauth/status",   mpOAuthHandler.Status)
	secured.DELETE("/me/mercadopago/oauth",       mpOAuthHandler.Disconnect)
	// Callback público — MP redireciona aqui após autorização
	api.GET("/mercadopago/oauth/callback",        mpOAuthHandler.Callback)

	// PagBank OAuth
	secured.GET("/me/pagbank/oauth/start",        pagbankOAuthHandler.Start)
	secured.GET("/me/pagbank/oauth/status",        pagbankOAuthHandler.Status)
	secured.DELETE("/me/pagbank/oauth",            pagbankOAuthHandler.Disconnect)
	// Callback público — PagBank redireciona aqui após autorização
	api.GET("/pagbank/oauth/callback",             pagbankOAuthHandler.Callback)
	// Webhook de pagamento PagBank
	api.POST("/webhooks/pagbank",                  pagbankWebhookHandler.Handle)

	// Google Calendar OAuth
	googleOAuthHandler := handlers.NewGoogleOAuthHandler(db, cfg)
	secured.GET("/me/google/oauth/start",  googleOAuthHandler.Start)
	secured.GET("/me/google/oauth/status", googleOAuthHandler.Status)
	secured.DELETE("/me/google/oauth",     googleOAuthHandler.Disconnect)
	api.GET("/google/oauth/callback",      googleOAuthHandler.Callback)

	secured.GET("/me/whatsapp/status",          whatsappHandler.Status)
	secured.POST("/me/whatsapp/connect",        whatsappHandler.Connect)
	secured.POST("/me/whatsapp/pairing-code",   whatsappHandler.PairingCode)
	secured.DELETE("/me/whatsapp/connect",      whatsappHandler.Disconnect)

	// Webhook público — Evolution API dispara aqui quando cliente manda mensagem
	api.POST("/webhooks/whatsapp", whatsappWebhookHandler.Receive)

	registerAppointmentRoutes(secured, appointmentHandler, internalAppointmentHandler,
		closureAdjustmentHandler, paymentHandler, operationalSummaryHandler,
		paymentReportHandler, orderHandler, closureListHandler, auditLogsHandler)

	registerAdminRoutes(secured, planHandler, dashboardHandler, financialHandler,
		dayPanelHandler, impactHandler, subscriptionHandler, billingHandler, imageHandler)

	// Endpoint de bypass de pagamento — dupla proteção:
	// 1) MPProvider != "mp"  (gateway real não configurado)
	// 2) AppEnv != "production" (variável de ambiente de ambiente)
	// Ambas precisam ser verdadeiras para o endpoint existir.
	if cfg.MPProvider != "mp" && cfg.AppEnv != "production" {
		devPaymentHandler := handlers.NewDevPaymentHandler(markMPPaymentAsPaidUC)
		api.POST("/dev/payments/:id/confirm", devPaymentHandler.ConfirmPayment)
		log.Println("[DEV] rota POST /api/dev/payments/:id/confirm ativa (mock mode)")
	}

	return auditDispatcher
}
