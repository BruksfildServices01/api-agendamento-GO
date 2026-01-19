package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

type ClientHandler struct {
	db *gorm.DB
}

func NewClientHandler(db *gorm.DB) *ClientHandler {
	return &ClientHandler{db: db}
}

// ======================================================
// LIST CLIENTS (BARBEIRO)
// ======================================================
func (h *ClientHandler) List(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	query := strings.ToLower(strings.TrimSpace(c.Query("query")))

	q := h.db.Where("barbershop_id = ?", barbershopID)

	if query != "" {
		like := "%" + query + "%"
		q = q.Where(
			"LOWER(name) LIKE ? OR phone LIKE ? OR LOWER(email) LIKE ?",
			like, like, like,
		)
	}

	var clients []models.Client
	if err := q.
		Order("created_at DESC").
		Find(&clients).Error; err != nil {

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed_to_list_clients",
		})
		return
	}

	c.JSON(http.StatusOK, clients)
}
