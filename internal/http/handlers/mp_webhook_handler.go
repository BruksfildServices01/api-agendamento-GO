package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mercadopago/sdk-go/pkg/config"
	"github.com/mercadopago/sdk-go/pkg/payment"
	"gorm.io/gorm"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/payment"
	paymentinfra "github.com/BruksfildServices01/barber-scheduler/internal/integration/payment"
	infraMP "github.com/BruksfildServices01/barber-scheduler/internal/integration/payment/mercadopago"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
	ucPayment "github.com/BruksfildServices01/barber-scheduler/internal/usecase/payment"
)

// MPWebhookHandler processa as notificações IPN do Mercado Pago e serve o
// endpoint de polling de status de pagamento para todos os providers.
type MPWebhookHandler struct {
	markMPPaid        *ucPayment.MarkMPPaymentAsPaid
	globalAccessToken string
	webhookSecret     string
	// requireSignature=true quando MPProvider=="mp".
	// Se true e webhookSecret vazio, o webhook é rejeitado sem processar.
	requireSignature bool
	db               *gorm.DB
	// registry é opcional — quando presente, habilita polling de status via
	// provider genérico (PagBank etc.) no endpoint CheckPaymentStatus.
	registry *paymentinfra.ProviderRegistry
}

func NewMPWebhookHandler(
	markMPPaid *ucPayment.MarkMPPaymentAsPaid,
	globalAccessToken string,
	webhookSecret string,
	requireSignature bool,
	db *gorm.DB,
) *MPWebhookHandler {
	return &MPWebhookHandler{
		markMPPaid:        markMPPaid,
		globalAccessToken: globalAccessToken,
		webhookSecret:     webhookSecret,
		requireSignature:  requireSignature,
		db:                db,
	}
}

// WithRegistry configura o ProviderRegistry para polling de status de pagamentos
// de providers não-MP (ex: PagBank). Retorna o próprio handler para encadeamento.
func (h *MPWebhookHandler) WithRegistry(r *paymentinfra.ProviderRegistry) *MPWebhookHandler {
	h.registry = r
	return h
}

// mpNotification é o corpo do IPN enviado pelo Mercado Pago.
type mpNotification struct {
	Action string `json:"action"`
	Type   string `json:"type"`
	Data   struct {
		ID string `json:"id"`
	} `json:"data"`
}

// Handle processa a notificação IPN do MP.
// POST /api/webhooks/mp  ou  POST /webhooks/mp
func (h *MPWebhookHandler) Handle(c *gin.Context) {
	var notif mpNotification
	if err := c.ShouldBindJSON(&notif); err != nil {
		c.Status(http.StatusOK)
		return
	}

	if notif.Type != "payment" || notif.Data.ID == "" {
		c.Status(http.StatusOK)
		return
	}

	// Valida assinatura HMAC.
	if h.webhookSecret != "" {
		xSig := c.GetHeader("x-signature")
		xReqID := c.GetHeader("x-request-id")
		if !infraMP.VerifyWebhookSignature(h.webhookSecret, xSig, xReqID, notif.Data.ID) {
			log.Printf("[MP WEBHOOK] assinatura inválida para pagamento %s — ignorado", notif.Data.ID)
			c.Status(http.StatusOK) // 200 para o MP não retentar
			return
		}
	} else if h.requireSignature {
		// Produção (requireSignature=true) sem secret configurado.
		// Bloqueia sem processar para evitar fraude por webhook forjado.
		log.Printf("[MP WEBHOOK] ALERTA: MP_WEBHOOK_SECRET não configurado em modo produção — webhook rejeitado sem processar")
		c.Status(http.StatusOK) // 200 para o MP não retentar
		return
	} else {
		log.Printf("[MP WEBHOOK] MP_WEBHOOK_SECRET não configurado — validação ignorada (modo dev)")
	}

	mpPaymentID := notif.Data.ID

	go func() {
		if err := h.processPayment(context.Background(), mpPaymentID); err != nil {
			log.Printf("[MP WEBHOOK] error processing payment %s: %v", mpPaymentID, err)
		}
	}()

	c.Status(http.StatusOK)
}

// resolveAccessToken descobre qual access token usar para buscar o pagamento no MP.
// Primeiro tenta encontrar o pagamento no banco pelo TxID e pegar o token da barbearia.
// Se não achar ou a barbearia não tiver token próprio, usa o token global.
func (h *MPWebhookHandler) resolveAccessToken(ctx context.Context, mpPaymentIDStr string) string {
	txid := "mp_pay:" + mpPaymentIDStr

	var p models.Payment
	err := h.db.WithContext(ctx).
		Where("txid = ?", txid).
		Select("barbershop_id").
		First(&p).Error

	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("[MP WEBHOOK] resolveAccessToken DB error: %v", err)
		}
		return h.globalAccessToken
	}

	var cfg models.BarbershopPaymentConfig
	err = h.db.WithContext(ctx).
		Where("barbershop_id = ?", p.BarbershopID).
		Select("mp_access_token").
		First(&cfg).Error

	if err != nil || strings.TrimSpace(cfg.MPAccessToken) == "" {
		return h.globalAccessToken
	}

	return cfg.MPAccessToken
}

// CheckPaymentStatus é chamado pelo frontend como fallback quando o webhook demora.
// Suporta Mercado Pago (via mp_payment_id / "mp_pay:" TxID) e providers genéricos
// como PagBank (via TxID + ProviderRegistry).
// GET /api/public/:slug/appointments/:id/payment/status
func (h *MPWebhookHandler) CheckPaymentStatus(c *gin.Context) {
	appointmentID, err := strconv.Atoi(c.Param("id"))
	if err != nil || appointmentID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_appointment_id"})
		return
	}

	slug := c.Param("slug")
	var shop models.Barbershop
	if err := h.db.WithContext(c.Request.Context()).
		Where("slug = ?", slug).Select("id").First(&shop).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "not_found"})
		return
	}

	var p models.Payment
	if err := h.db.WithContext(c.Request.Context()).
		Where("barbershop_id = ? AND appointment_id = ?", shop.ID, uint(appointmentID)).
		First(&p).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"status": "no_payment"})
		return
	}

	// Estados finais — sem chamada externa.
	if p.Status == "paid" {
		c.JSON(http.StatusOK, gin.H{"status": "confirmed"})
		return
	}
	if p.Status == "expired" {
		c.JSON(http.StatusOK, gin.H{"status": "expired"})
		return
	}

	// Tenta o caminho Mercado Pago primeiro:
	// MPPaymentID pode ser nil — pagamentos transparentes gravam apenas TxID ("mp_pay:<id>").
	var mpPaymentIDStr string
	if p.MPPaymentID != nil {
		mpPaymentIDStr = strconv.FormatInt(*p.MPPaymentID, 10)
	} else if p.TxID != nil && strings.HasPrefix(*p.TxID, "mp_pay:") {
		mpPaymentIDStr = strings.TrimPrefix(*p.TxID, "mp_pay:")
	}

	if mpPaymentIDStr != "" {
		if err := h.processPayment(c.Request.Context(), mpPaymentIDStr); err != nil {
			log.Printf("[CheckPaymentStatus] appointment=%d mp_payment=%s error=%v", appointmentID, mpPaymentIDStr, err)
		}
		h.replyWithCurrentStatus(c, &p)
		return
	}

	// Caminho genérico: PagBank e outros providers.
	// TxID contém o provider payment ID (ex: "QRC_XXXXX", "CHAR_XXXXX").
	if p.TxID != nil && *p.TxID != "" && h.registry != nil {
		if err := h.checkStatusViaRegistry(c.Request.Context(), shop.ID, &p); err != nil {
			log.Printf("[CheckPaymentStatus] appointment=%d provider_id=%s error=%v",
				appointmentID, *p.TxID, err)
		}
	}

	h.replyWithCurrentStatus(c, &p)
}

// replyWithCurrentStatus relê o payment do banco e responde com o status atual.
// Converte "paid" → "confirmed" para manter o contrato do frontend.
func (h *MPWebhookHandler) replyWithCurrentStatus(c *gin.Context, p *models.Payment) {
	var updated models.Payment
	if err := h.db.WithContext(c.Request.Context()).
		Where("id = ?", p.ID).Select("status").First(&updated).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"status": string(p.Status)})
		return
	}
	status := string(updated.Status)
	if status == "paid" {
		status = "confirmed"
	}
	c.JSON(http.StatusOK, gin.H{"status": status})
}

// checkStatusViaRegistry consulta o status do pagamento via PaymentGateway genérico.
// Usado para providers não-MP (PagBank etc.) quando o TxID não tem prefixo "mp_pay:".
// Se o provider retornar aprovado, chama markMPPaid para confirmar o payment interno.
//
// Prioridade de identificação:
//   1. payment.ProviderPaymentID — ID externo puro (sem prefixo), gravado na criação.
//   2. payment.TxID              — fallback para payments antigos sem ProviderPaymentID.
//
// Prioridade de seleção do gateway:
//   1. payment.Provider — consulta o provider específico que criou o payment, evitando
//      que uma troca de provider posterior interfira no polling de payments antigos.
//   2. provider ativo mais recente via TransparentGatewayFor — fallback para payments
//      antigos (criados antes do campo provider existir) sem provider registrado.
func (h *MPWebhookHandler) checkStatusViaRegistry(
	ctx context.Context,
	shopID uint,
	p *models.Payment,
) error {
	// Determina o ID externo a ser consultado no provider.
	providerPaymentID := ""
	if p.ProviderPaymentID != nil && *p.ProviderPaymentID != "" {
		providerPaymentID = *p.ProviderPaymentID
	} else if p.TxID != nil {
		providerPaymentID = *p.TxID
	}
	if providerPaymentID == "" {
		return nil
	}

	// Seleciona o gateway: usa o provider salvo no payment quando disponível.
	var gw domain.TransparentGateway
	var err error

	if p.Provider != nil && *p.Provider != "" {
		// Provider registrado — consulta o gateway específico, independentemente de
		// qual provider está ativo na barbearia agora.
		gw, err = h.registry.GatewayForProvider(ctx, shopID, *p.Provider)
	} else {
		// Fallback para payments antigos sem campo provider:
		// usa o provider mais recentemente ativo (comportamento original).
		var paymentCfg models.BarbershopPaymentConfig
		_ = h.db.WithContext(ctx).Where("barbershop_id = ?", shopID).First(&paymentCfg).Error
		paymentCfg.BarbershopID = shopID
		gw, err = h.registry.TransparentGatewayFor(ctx, paymentCfg)
	}
	if err != nil {
		return fmt.Errorf("registry: %w", err)
	}

	// Duck typing: apenas gateways que implementam GetPaymentStatus são consultados.
	// pagbank.Gateway e mp.Gateway implementam; gateways sem suporte a polling são ignorados.
	type statusChecker interface {
		GetPaymentStatus(ctx context.Context, providerPaymentID string) (domain.ProviderPaymentStatus, error)
	}
	checker, ok := gw.(statusChecker)
	if !ok {
		return nil
	}

	status, err := checker.GetPaymentStatus(ctx, providerPaymentID)
	if err != nil {
		return fmt.Errorf("GetPaymentStatus(%s): %w", providerPaymentID, err)
	}

	if status != domain.ProviderStatusApproved {
		return nil
	}

	externalRef := strconv.FormatUint(uint64(p.ID), 10)
	return h.markMPPaid.Execute(ctx, externalRef, providerPaymentID)
}

func (h *MPWebhookHandler) processPayment(ctx context.Context, mpPaymentIDStr string) error {
	accessToken := h.resolveAccessToken(ctx, mpPaymentIDStr)
	if accessToken == "" {
		return fmt.Errorf("no MP access token available")
	}

	cfg, err := config.New(accessToken)
	if err != nil {
		return fmt.Errorf("mp config error: %w", err)
	}
	paymentClient := payment.NewClient(cfg)

	mpPaymentID, err := strconv.Atoi(mpPaymentIDStr)
	if err != nil {
		return fmt.Errorf("invalid payment id %q: %w", mpPaymentIDStr, err)
	}

	resp, err := paymentClient.Get(ctx, mpPaymentID)
	if err != nil {
		return fmt.Errorf("failed to get MP payment: %w", err)
	}

	if resp.Status != "approved" {
		return nil
	}

	if resp.ExternalReference == "" {
		return fmt.Errorf("empty external_reference for MP payment %d", mpPaymentID)
	}

	return h.markMPPaid.Execute(ctx, resp.ExternalReference, mpPaymentIDStr)
}
