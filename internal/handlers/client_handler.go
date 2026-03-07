package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
	ucMetrics "github.com/BruksfildServices01/barber-scheduler/internal/usecase/metrics"
)

type ClientWithCategoryResponse struct {
	models.Client
	Category string `json:"category"`
}

type ClientHandler struct {
	db                     *gorm.DB
	getClientsWithCategory *ucMetrics.GetClientsWithCategory
}

func NewClientHandler(
	db *gorm.DB,
	getClientsWithCategory *ucMetrics.GetClientsWithCategory,
) *ClientHandler {
	return &ClientHandler{
		db:                     db,
		getClientsWithCategory: getClientsWithCategory,
	}
}

// ======================================================
// LIST CLIENTS (BARBEIRO) — Sprint 6
// ======================================================
func (h *ClientHandler) List(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)
	query := strings.ToLower(strings.TrimSpace(c.Query("query")))

	// 1️⃣ Busca clientes (dados básicos)
	q := h.db.Where("barbershop_id = ?", barbershopID)

	if query != "" {
		like := "%" + query + "%"
		q = q.Where(
			"LOWER(name) LIKE ? OR phone LIKE ? OR LOWER(email) LIKE ?",
			like, like, like,
		)
	}

	var clients []models.Client
	if err := q.Order("created_at DESC").Find(&clients).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed_to_list_clients",
		})
		return
	}

	// 2️⃣ Busca categorias em bulk (metrics)
	categories, err :=
		h.getClientsWithCategory.Execute(
			c.Request.Context(),
			barbershopID,
		)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed_to_resolve_client_category",
		})
		return
	}

	categoryByClient := make(map[uint]string, len(categories))
	for _, c := range categories {
		categoryByClient[c.ClientID] = string(c.Category)
	}

	// 3️⃣ Enriquecimento final
	out := make([]ClientWithCategoryResponse, 0, len(clients))
	for _, client := range clients {
		out = append(out, ClientWithCategoryResponse{
			Client:   client,
			Category: categoryByClient[client.ID],
		})
	}

	c.JSON(http.StatusOK, out)
}
