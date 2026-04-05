package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
	ucMetrics "github.com/BruksfildServices01/barber-scheduler/internal/usecase/metrics"
	ucSubscription "github.com/BruksfildServices01/barber-scheduler/internal/usecase/subscription"
)

type ClientListItemResponse struct {
	models.Client
	Category string `json:"category"`
	Premium  bool   `json:"premium"`
}

type ClientHandler struct {
	db                     *gorm.DB
	getClientsWithCategory *ucMetrics.GetClientsWithCategory
	getActiveSubscription  *ucSubscription.GetActiveSubscription
}

func NewClientHandler(
	db *gorm.DB,
	getClientsWithCategory *ucMetrics.GetClientsWithCategory,
	getActiveSubscription *ucSubscription.GetActiveSubscription,
) *ClientHandler {
	return &ClientHandler{
		db:                     db,
		getClientsWithCategory: getClientsWithCategory,
		getActiveSubscription:  getActiveSubscription,
	}
}

// ======================================================
// LIST CLIENTS
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
	if err := q.Order("created_at DESC").Find(&clients).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed_to_list_clients",
		})
		return
	}

	categories, err := h.getClientsWithCategory.Execute(
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
	for _, item := range categories {
		categoryByClient[item.ClientID] = string(item.Category)
	}

	out := make([]ClientListItemResponse, 0, len(clients))
	for _, client := range clients {
		premium := false

		if h.getActiveSubscription != nil {
			sub, err := h.getActiveSubscription.Execute(
				c.Request.Context(),
				barbershopID,
				client.ID,
			)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "failed_to_resolve_client_premium",
				})
				return
			}
			premium = sub != nil
		}

		out = append(out, ClientListItemResponse{
			Client:   client,
			Category: categoryByClient[client.ID],
			Premium:  premium,
		})
	}

	c.JSON(http.StatusOK, out)
}
