package handlers

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/infra/notification"
	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

// WhatsAppHandler gerencia conexões WhatsApp via Evolution API.
// O barbeiro só escaneia o QR code — toda a configuração técnica
// (URL, instância, webhook) é feita automaticamente pelo sistema.
type WhatsAppHandler struct {
	db           *gorm.DB
	evolutionURL string
	evolutionKey string
	backendURL   string // para montar a URL do webhook
}

func NewWhatsAppHandler(db *gorm.DB, evolutionURL, evolutionKey, backendURL string) *WhatsAppHandler {
	return &WhatsAppHandler{
		db:           db,
		evolutionURL: evolutionURL,
		evolutionKey: evolutionKey,
		backendURL:   backendURL,
	}
}

func (h *WhatsAppHandler) client() *notification.EvolutionClient {
	return notification.NewEvolutionClient(h.evolutionURL, h.evolutionKey)
}

func instanceName(barbershopID uint, barberID *uint) string {
	if barberID != nil {
		return fmt.Sprintf("bs%d_b%d", barbershopID, *barberID)
	}
	return fmt.Sprintf("bs%d", barbershopID)
}

// Status retorna o estado atual da conexão WhatsApp da barbearia.
// GET /api/me/whatsapp/status
func (h *WhatsAppHandler) Status(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	var inst models.BarbershopWhatsAppInstance
	err := h.db.WithContext(c.Request.Context()).
		Where("barbershop_id = ? AND barber_id IS NULL", barbershopID).
		First(&inst).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusOK, gin.H{
			"connected":     false,
			"status":        "disconnected",
			"instance_name": "",
			"phone":         "",
		})
		return
	}
	if err != nil {
		httperr.Internal(c, "internal_error", err.Error())
		return
	}

	// Verifica estado real na Evolution API
	state := "disconnected"
	if h.evolutionURL != "" {
		ctx := c.Request.Context()
		if s, err := h.client().ConnectionState(ctx, inst.InstanceName); err == nil {
			state = s
		}
	}

	// Sincroniza status no banco
	newStatus := "disconnected"
	if state == "open" {
		newStatus = "connected"
	}
	if inst.Status != newStatus {
		h.db.WithContext(c.Request.Context()).
			Model(&inst).Update("status", newStatus)
		inst.Status = newStatus
	}

	c.JSON(http.StatusOK, gin.H{
		"connected":     state == "open",
		"status":        inst.Status,
		"instance_name": inst.InstanceName,
		"phone":         inst.Phone,
	})
}

// Connect cria a instância e retorna o QR code para o barbeiro escanear.
// POST /api/me/whatsapp/connect
func (h *WhatsAppHandler) Connect(c *gin.Context) {
	if h.evolutionURL == "" {
		httperr.BadRequest(c, "evolution_not_configured", "Evolution API não configurada no servidor.")
		return
	}

	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)
	name := instanceName(barbershopID, nil)

	// Garante registro no banco
	var inst models.BarbershopWhatsAppInstance
	err := h.db.WithContext(c.Request.Context()).
		Where("barbershop_id = ? AND barber_id IS NULL", barbershopID).
		First(&inst).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		inst = models.BarbershopWhatsAppInstance{
			BarbershopID: barbershopID,
			InstanceName: name,
			Status:       "connecting",
		}
		if err := h.db.WithContext(c.Request.Context()).Create(&inst).Error; err != nil {
			httperr.Internal(c, "internal_error", err.Error())
			return
		}
	}

	ctx := c.Request.Context()
	client := h.client()

	// Tenta criar instância na Evolution API (ignora erro se já existe)
	_ = client.CreateInstance(ctx, name)

	// Configura webhook automaticamente
	webhookURL := h.backendURL + "/api/webhooks/whatsapp"
	_ = client.SetWebhook(ctx, name, webhookURL)

	// Busca QR code
	qr, err := client.GetQRCode(ctx, name)
	if err != nil {
		log.Printf("[WhatsApp] GetQRCode error: %v", err)
		httperr.Internal(c, "qrcode_failed", "Não foi possível gerar o QR code.")
		return
	}
	log.Printf("[WhatsApp] QR code base64 len=%d code_len=%d", len(qr.Base64), len(qr.Code))

	// Atualiza status para connecting
	h.db.WithContext(ctx).Model(&inst).Updates(map[string]any{
		"status":     "connecting",
		"updated_at": time.Now(),
	})

	c.JSON(http.StatusOK, gin.H{
		"qrcode_base64": qr.Base64,
		"qrcode_code":   qr.Code,
		"instance_name": name,
	})
}

// PairingCode gera um código de 8 dígitos para conectar pelo número de telefone.
// O barbeiro digita o código no WhatsApp em vez de escanear o QR code.
// POST /api/me/whatsapp/pairing-code
func (h *WhatsAppHandler) PairingCode(c *gin.Context) {
	if h.evolutionURL == "" {
		httperr.BadRequest(c, "evolution_not_configured", "Evolution API não configurada no servidor.")
		return
	}

	var req struct {
		Phone string `json:"phone" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid_payload", "Número de telefone obrigatório.")
		return
	}

	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)
	name := instanceName(barbershopID, nil)

	// Garante que a instância existe
	var inst models.BarbershopWhatsAppInstance
	err := h.db.WithContext(c.Request.Context()).
		Where("barbershop_id = ? AND barber_id IS NULL", barbershopID).
		First(&inst).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		inst = models.BarbershopWhatsAppInstance{
			BarbershopID: barbershopID,
			InstanceName: name,
			Status:       "connecting",
		}
		if err := h.db.WithContext(c.Request.Context()).Create(&inst).Error; err != nil {
			httperr.Internal(c, "internal_error", err.Error())
			return
		}
	}

	ctx := c.Request.Context()
	client := h.client()

	// Cria instância (ignora erro se já existe)
	_ = client.CreateInstance(ctx, name)

	// Configura webhook
	_ = client.SetWebhook(ctx, name, h.backendURL+"/api/webhooks/whatsapp")

	// Solicita pairing code
	code, err := client.GetPairingCode(ctx, name, req.Phone)
	if err != nil {
		httperr.Internal(c, "pairing_code_failed", "Não foi possível gerar o código. Tente o QR code.")
		return
	}

	h.db.WithContext(ctx).Model(&inst).Updates(map[string]any{
		"status":     "connecting",
		"phone":      req.Phone,
		"updated_at": time.Now(),
	})

	c.JSON(http.StatusOK, gin.H{"code": code})
}

// Disconnect desconecta o WhatsApp e remove a instância.
// DELETE /api/me/whatsapp/connect
func (h *WhatsAppHandler) Disconnect(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	var inst models.BarbershopWhatsAppInstance
	err := h.db.WithContext(c.Request.Context()).
		Where("barbershop_id = ? AND barber_id IS NULL", barbershopID).
		First(&inst).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.Status(http.StatusNoContent)
		return
	}
	if err != nil {
		httperr.Internal(c, "internal_error", err.Error())
		return
	}

	// Remove instância da Evolution API
	if h.evolutionURL != "" {
		_ = h.client().DeleteInstance(c.Request.Context(), inst.InstanceName)
	}

	// Remove do banco
	h.db.WithContext(c.Request.Context()).Delete(&inst)
	c.Status(http.StatusNoContent)
}
