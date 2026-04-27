package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/BruksfildServices01/barber-scheduler/internal/config"
	"github.com/BruksfildServices01/barber-scheduler/internal/handlers"
	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
)


// registerPublicRoutes registra todas as rotas acessíveis sem autenticação.
func registerPublicRoutes(
	api *gin.RouterGroup,
	cfg *config.Config,
	pub *handlers.PublicHandler,
	checkout *handlers.PublicCheckoutHandler,
	mpPayment *handlers.MPPaymentHandler,
	transparent *handlers.TransparentPaymentHandler,
	pubSubscription *handlers.PublicSubscriptionHandler,
	ticket *handlers.PublicTicketHandler,
) {
	g := api.Group("/public")

	g.GET("/:slug/info", pub.GetInfo)
	g.GET("/:slug/services", pub.ListServices)
	g.GET("/:slug/products", pub.ListProducts)
	g.GET("/:slug/services/:id/suggestion", pub.GetServiceSuggestion)
	g.GET("/:slug/availability", pub.AvailabilityForClient)
	g.POST("/:slug/appointments", pub.CreateAppointment)

	g.GET("/:slug/cart", pub.GetCart)
	g.POST("/:slug/cart/items", pub.AddCartItem)
	g.DELETE("/:slug/cart/items/:productId", pub.RemoveCartItem)
	g.POST("/:slug/cart/checkout", pub.CheckoutCart)
	g.POST("/:slug/checkout", checkout.Checkout)

	g.POST(
		"/:slug/appointments/:id/payment/mp",
		middleware.NewRateLimitByKey(func(c *gin.Context) string {
			return middleware.ClientIPKey(c) + ":" + c.Param("slug")
		}, 30, 60, cfg.RedisURL), // 30 req/minuto
		mpPayment.CreatePreference,
	)
	g.POST(
		"/:slug/appointments/:id/payment/transparent",
		middleware.NewRateLimitByKey(func(c *gin.Context) string {
			return middleware.ClientIPKey(c) + ":" + c.Param("slug")
		}, 30, 60, cfg.RedisURL), // 30 req/minuto
		transparent.CreatePayment,
	)

	g.GET("/:slug/plans", pubSubscription.ListPlans)
	g.GET("/:slug/subscribers/lookup", pubSubscription.LookupSubscriber)
	g.POST(
		"/:slug/subscriptions/purchase",
		middleware.NewRateLimitByKey(func(c *gin.Context) string {
			return middleware.ClientIPKey(c) + ":" + c.Param("slug")
		}, 10, 60, cfg.RedisURL), // 10 req/minuto
		pubSubscription.Purchase,
	)
	g.GET("/:slug/subscriptions/:id/payment/status", pubSubscription.PaymentStatus)

	g.GET("/ticket/:token", ticket.View)
	g.DELETE("/ticket/:token", ticket.Cancel)
	g.PATCH("/ticket/:token", ticket.Reschedule)
}

// registerWebhookAndAuthRoutes registra webhooks públicos e rotas de autenticação.
func registerWebhookAndAuthRoutes(
	r *gin.Engine,
	api *gin.RouterGroup,
	cfg *config.Config,
	mpWebhook *handlers.MPWebhookHandler,
	billing *handlers.BillingHandler,
	auth *handlers.AuthHandler,
	pwReset *handlers.PasswordResetHandler,
) {
	// O MP envia para /webhooks/mp — registrado em ambos os prefixos por compatibilidade.
	api.POST("/webhooks/mp", middleware.MaxBodySize(64*1024), mpWebhook.Handle)
	r.POST("/webhooks/mp", middleware.MaxBodySize(64*1024), mpWebhook.Handle)

	// Rate limit por IP: proteção contra brute force e spam de registro.
	ipKey := func(c *gin.Context) string { return middleware.ClientIPKey(c) }

	// Endpoints de auth: fail-closed — se Redis cair, bloqueia para evitar brute force
	api.POST("/auth/register",
		middleware.NewRateLimitByKeyStrict(ipKey, 5, 3600, cfg.RedisURL), // 5/hora
		auth.Register,
	)
	api.POST("/auth/login",
		middleware.NewRateLimitByKeyStrict(ipKey, 10, 300, cfg.RedisURL), // 10/5min
		auth.Login,
	)
	api.POST("/auth/password-reset/request",
		middleware.NewRateLimitByKeyStrict(ipKey, 5, 300, cfg.RedisURL), // 5/5min
		pwReset.Request,
	)
	api.POST("/auth/password-reset/confirm",
		middleware.NewRateLimitByKey(ipKey, 10, 300, cfg.RedisURL), // 10/5min
		pwReset.Confirm,
	)

	api.POST("/billing/webhook", middleware.MaxBodySize(64*1024), billing.Webhook)
}

// registerCatalogRoutes registra rotas de catálogo: barbershop, serviços, produtos e horários.
func registerCatalogRoutes(
	g *gin.RouterGroup,
	me *handlers.MeHandler,
	barbershop *handlers.BarbershopHandler,
	service *handlers.ServiceHandler,
	serviceCategory *handlers.ServiceCategoryHandler,
	serviceSuggestion *handlers.ServiceSuggestionHandler,
	product *handlers.ProductHandler,
	workingHours *handlers.WorkingHoursHandler,
	scheduleOverride *handlers.ScheduleOverrideHandler,
) {
	g.GET("/me", me.GetMe)
	g.POST("/me/tours/:screenId/seen", me.MarkTourSeen)
	g.GET("/me/barbershop", barbershop.GetMeBarbershop)
	g.PUT("/me/barbershop", middleware.RequireOwner, barbershop.UpdateMeBarbershop)
	g.PATCH("/me/barbershop/slug", middleware.RequireOwner, barbershop.UpdateSlug)

	g.GET("/me/services", service.List)
	g.POST("/me/services", middleware.RequireOwner, service.Create)
	g.PUT("/me/services/:id", middleware.RequireOwner, service.Update)
	g.DELETE("/me/services/:id", middleware.RequireOwner, service.Delete)

	g.GET("/me/service-categories", serviceCategory.List)
	g.POST("/me/service-categories", middleware.RequireOwner, serviceCategory.Create)
	g.PUT("/me/service-categories/:id", middleware.RequireOwner, serviceCategory.Update)
	g.DELETE("/me/service-categories/:id", middleware.RequireOwner, serviceCategory.Delete)

	g.GET("/me/services/:id/suggestion", serviceSuggestion.Get)
	g.PUT("/me/services/:id/suggestion", middleware.RequireOwner, serviceSuggestion.Set)
	g.DELETE("/me/services/:id/suggestion", middleware.RequireOwner, serviceSuggestion.Remove)

	g.GET("/me/products", product.List)
	g.POST("/me/products", middleware.RequireOwner, product.Create)
	g.PUT("/me/products/:id", middleware.RequireOwner, product.Update)
	g.DELETE("/me/products/:id", middleware.RequireOwner, product.Delete)

	g.GET("/me/working-hours", workingHours.Get)
	g.PUT("/me/working-hours", workingHours.Update)

	g.GET("/me/schedule-overrides", scheduleOverride.List)
	g.PUT("/me/schedule-overrides", scheduleOverride.Upsert)
	g.DELETE("/me/schedule-overrides/:id", scheduleOverride.Delete)
}

// registerClientRoutes registra rotas de CRM e políticas de pagamento.
func registerClientRoutes(
	g *gin.RouterGroup,
	client *handlers.ClientHandler,
	clientHistory *handlers.ClientHistoryHandler,
	clientCategory *handlers.ClientCategoryHandler,
	clientCategoryOverride *handlers.ClientCategoryOverrideHandler,
	crm *handlers.CRMHandler,
	clientAnonymize *handlers.ClientAnonymizeHandler,
	paymentPolicy *handlers.PaymentPolicyHandler,
) {
	g.GET("/me/clients", client.List)
	g.GET("/me/clients/:id/crm", crm.Get)
	g.GET("/me/clients/:id/history", clientHistory.Get)
	g.GET("/me/clients/:id/category", clientCategory.Get)
	g.PUT("/me/clients/:id/category", clientCategoryOverride.Update)
	// LGPD — anonimização de dados pessoais a pedido do titular
	g.POST("/me/clients/:id/anonymize", middleware.RequireOwner, clientAnonymize.Anonymize)

	g.GET("/me/payment-policies", middleware.RequireOwner, paymentPolicy.Get)
	g.PUT("/me/payment-policies", middleware.RequireOwner, paymentPolicy.Update)
}

// registerAppointmentRoutes registra agendamentos, pagamentos, pedidos e fechamentos.
func registerAppointmentRoutes(
	g *gin.RouterGroup,
	appt *handlers.AppointmentHandler,
	internalAppt *handlers.InternalAppointmentHandler,
	closureAdj *handlers.ClosureAdjustmentHandler,
	payment *handlers.PaymentHandler,
	opSummary *handlers.OperationalSummaryHandler,
	paymentReport *handlers.PaymentReportHandler,
	order *handlers.OrderHandler,
	closure *handlers.ClosureListHandler,
	auditLogs *handlers.AuditLogsHandler,
) {
	g.POST("/me/appointments", appt.Create)
	g.PUT("/me/appointments/:id/complete", appt.Complete)
	g.PUT("/me/appointments/:id/cancel", appt.Cancel)
	g.PUT("/me/appointments/:id/no-show", appt.MarkNoShow)
	g.POST("/me/appointments/:id/closure/adjustment", closureAdj.Create)
	g.GET("/me/appointments/date", appt.ListByDate)
	g.GET("/me/appointments/month", appt.ListByMonth)

	g.POST("/me/internal-appointments", internalAppt.Create)

	g.GET("/me/payments", payment.List)
	g.GET("/me/payments/cash-due", payment.CashDue)
	g.GET("/me/summary", opSummary.Get)
	g.GET("/me/payments/summary", paymentReport.Summary)

	g.POST("/me/orders", order.Create)
	g.GET("/me/orders", order.List)
	g.GET("/me/orders/:id", order.GetByID)

	g.GET("/me/closures", closure.List)
	g.GET("/me/closures/:id", closure.GetByID)

	g.GET("/me/audit-logs", middleware.RequireOwner, auditLogs.List)
}

// registerAdminRoutes registra rotas de gestão do negócio (owner only na maioria).
func registerAdminRoutes(
	g *gin.RouterGroup,
	plan *handlers.PlanHandler,
	dashboard *handlers.DashboardHandler,
	financial *handlers.FinancialHandler,
	dayPanel *handlers.DayPanelHandler,
	impact *handlers.ImpactHandler,
	subscription *handlers.SubscriptionHandler,
	billing *handlers.BillingHandler,
	image *handlers.ImageHandler,
) {
	g.POST("/me/plans", middleware.RequireOwner, plan.Create)
	g.GET("/me/plans", plan.List)
	g.PUT("/me/plans/:id", middleware.RequireOwner, plan.Update)
	g.PATCH("/me/plans/:id/active", middleware.RequireOwner, plan.SetActive)
	g.DELETE("/me/plans/:id", middleware.RequireOwner, plan.Delete)

	g.GET("/me/dashboard", middleware.RequireOwner, dashboard.Get)
	g.GET("/me/financial", middleware.RequireOwner, financial.Get)
	g.GET("/me/day-panel", dayPanel.Get)
	g.GET("/me/impact", middleware.RequireOwner, impact.Get)

	g.GET("/me/subscriptions", subscription.List)
	g.POST("/me/subscriptions", middleware.RequireOwner, subscription.Activate)
	g.DELETE("/me/subscriptions/:clientID", middleware.RequireOwner, subscription.Cancel)
	g.GET("/me/subscriptions/:clientID", subscription.GetActive)

	g.GET("/me/billing/status", billing.Status)
	g.POST("/me/billing/checkout", middleware.RequireOwner, billing.Checkout)
	g.POST("/me/billing/pay", middleware.RequireOwner, billing.Pay)

	if image != nil {
		g.POST("/me/services/:id/images", middleware.RequireOwner, image.AddServiceImage)
		g.DELETE("/me/services/:id/images/:imageId", middleware.RequireOwner, image.DeleteServiceImage)
		g.PUT("/me/products/:id/image", middleware.RequireOwner, image.SetProductImage)
		g.DELETE("/me/products/:id/image", middleware.RequireOwner, image.DeleteProductImage)
		g.PUT("/me/profile/photo", middleware.RequireOwner, image.SetProfilePhoto)
		g.DELETE("/me/profile/photo", middleware.RequireOwner, image.DeleteProfilePhoto)
	}
}
