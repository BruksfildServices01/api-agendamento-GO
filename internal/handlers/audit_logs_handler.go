package handlers

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

// ======================================================
// HANDLER
// ======================================================

type AuditLogsHandler struct {
	db *gorm.DB
}

func NewAuditLogsHandler(db *gorm.DB) *AuditLogsHandler {
	return &AuditLogsHandler{db: db}
}

func (h *AuditLogsHandler) List(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	action := c.Query("action")
	entity := c.Query("entity")
	fromStr := c.Query("from")
	toStr := c.Query("to")

	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "50")

	page, _ := strconv.Atoi(pageStr)
	if page <= 0 {
		page = 1
	}

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	offset := (page - 1) * limit

	// --------------------------------------------------
	// Query base (sempre protegido por barbershop)
	// --------------------------------------------------

	q := h.db.
		Model(&models.AuditLog{}).
		Where("barbershop_id = ?", barbershopID)

	// --------------------------------------------------
	// Filtros opcionais
	// --------------------------------------------------

	if action != "" {
		q = q.Where("action = ?", action)
	}

	if entity != "" {
		q = q.Where("entity = ?", entity)
	}

	if fromStr != "" {
		if from, err := time.Parse("2006-01-02", fromStr); err == nil {
			q = q.Where("created_at >= ?", from)
		}
	}

	if toStr != "" {
		if to, err := time.Parse("2006-01-02", toStr); err == nil {
			q = q.Where("created_at <= ?", to.Add(24*time.Hour))
		}
	}

	// --------------------------------------------------
	// Total
	// --------------------------------------------------

	var total int64
	if err := q.Count(&total).Error; err != nil {
		httperr.Internal(c, "audit_count_failed", "Erro ao contar logs.")
		return
	}

	// --------------------------------------------------
	// Listagem
	// --------------------------------------------------

	var logs []models.AuditLog
	if err := q.
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&logs).Error; err != nil {

		httperr.Internal(c, "audit_list_failed", "Erro ao listar logs.")
		return
	}

	// --------------------------------------------------
	// Response
	// --------------------------------------------------

	c.JSON(200, gin.H{
		"page":  page,
		"limit": limit,
		"total": total,
		"logs":  logs,
	})
}
