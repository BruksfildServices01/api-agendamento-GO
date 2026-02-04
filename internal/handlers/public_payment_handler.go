package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
	ucPayment "github.com/BruksfildServices01/barber-scheduler/internal/usecase/payment"
)

type PublicPaymentHandler struct {
	db        *gorm.DB
	createPix *ucPayment.CreatePixPayment
}

func NewPublicPaymentHandler(
	db *gorm.DB,
	createPix *ucPayment.CreatePixPayment,
) *PublicPaymentHandler {
	return &PublicPaymentHandler{
		db:        db,
		createPix: createPix,
	}
}

// ======================================================
// POST /api/public/:slug/appointments/:id/payment/pix
// ======================================================
func (h *PublicPaymentHandler) CreatePix(c *gin.Context) {
	slug := c.Param("slug")

	// --------------------------------------------------
	// 1️⃣ Resolve barbershop pelo slug (API pública)
	// --------------------------------------------------
	var shop models.Barbershop
	if err := h.db.
		Where("slug = ?", slug).
		First(&shop).Error; err != nil {

		httperr.NotFound(
			c,
			"barbershop_not_found",
			"Barbearia não encontrada.",
		)
		return
	}

	// --------------------------------------------------
	// 2️⃣ Appointment ID
	// --------------------------------------------------
	appointmentID, err := strconv.Atoi(c.Param("id"))
	if err != nil || appointmentID <= 0 {
		httperr.BadRequest(
			c,
			"invalid_appointment_id",
			"Agendamento inválido.",
		)
		return
	}

	// --------------------------------------------------
	// 3️⃣ Body (amount)
	// --------------------------------------------------
	var body struct {
		Amount float64 `json:"amount" binding:"required"`
	}

	if err := c.ShouldBindJSON(&body); err != nil || body.Amount <= 0 {
		httperr.BadRequest(
			c,
			"invalid_request",
			"Valor inválido.",
		)
		return
	}

	// --------------------------------------------------
	// 4️⃣ Executa use case
	// --------------------------------------------------
	payment, pixCharge, err := h.createPix.Execute(
		c.Request.Context(),
		shop.ID,
		uint(appointmentID),
		body.Amount,
	)

	if err != nil {
		switch {
		case httperr.IsBusiness(err, "payment_already_exists"):
			httperr.BadRequest(
				c,
				"payment_already_exists",
				"Pagamento já foi gerado para este agendamento.",
			)
		default:
			httperr.Internal(
				c,
				"payment_creation_failed",
				"Erro ao gerar pagamento.",
			)
		}
		return
	}

	// --------------------------------------------------
	// 5️⃣ Response
	// --------------------------------------------------
	c.JSON(http.StatusCreated, gin.H{
		"payment": payment,
		"pix": gin.H{
			"txid":       pixCharge.TxID,
			"qr_code":    pixCharge.QRCode,
			"expires_at": pixCharge.ExpiresAt,
		},
	})
}
