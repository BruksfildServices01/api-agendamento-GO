package handlers

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

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

type ClientListResponse struct {
	Clients        []ClientListItemResponse `json:"clients"`
	Total          int64                    `json:"total"`
	Page           int                      `json:"page"`
	PerPage        int                      `json:"per_page"`
	TotalPages     int                      `json:"total_pages"`
	CategoryCounts map[string]int           `json:"category_counts"`
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
	ctx := c.Request.Context()

	query := strings.ToLower(strings.TrimSpace(c.Query("q")))
	if query == "" {
		// also accept legacy "query" param
		query = strings.ToLower(strings.TrimSpace(c.Query("query")))
	}
	categoryFilter := c.Query("category")

	page := 1
	if p, err := strconv.Atoi(c.Query("page")); err == nil && p > 0 {
		page = p
	}

	perPage := 30
	if pp, err := strconv.Atoi(c.Query("per_page")); err == nil && pp > 0 && pp <= 100 {
		perPage = pp
	}

	// 1. Load all client metrics to compute effective categories.
	categories, err := h.getClientsWithCategory.Execute(ctx, barbershopID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_to_resolve_client_category"})
		return
	}

	categoryByClient := make(map[uint]string, len(categories))
	clientsByCategory := make(map[string][]uint)
	for _, item := range categories {
		cat := string(item.Category)
		categoryByClient[item.ClientID] = cat
		clientsByCategory[cat] = append(clientsByCategory[cat], item.ClientID)
	}

	categoryCounts := map[string]int{
		"at_risk": len(clientsByCategory["at_risk"]),
		"new":     len(clientsByCategory["new"]),
		"trusted": len(clientsByCategory["trusted"]),
		"regular": len(clientsByCategory["regular"]),
	}

	// 2. Build base query.
	q := h.db.WithContext(ctx).Model(&models.Client{}).Where("barbershop_id = ?", barbershopID)

	if query != "" {
		like := "%" + query + "%"
		q = q.Where("LOWER(name) LIKE ? OR phone LIKE ? OR LOWER(email) LIKE ?", like, like, like)
	}

	if categoryFilter != "" {
		ids := clientsByCategory[categoryFilter]
		if len(ids) == 0 {
			c.JSON(http.StatusOK, ClientListResponse{
				Clients:        []ClientListItemResponse{},
				Total:          0,
				Page:           page,
				PerPage:        perPage,
				TotalPages:     0,
				CategoryCounts: categoryCounts,
			})
			return
		}
		q = q.Where("id IN ?", ids)
	}

	// 3. Count total matching records.
	var total int64
	if err := q.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_to_count_clients"})
		return
	}

	totalPages := int(math.Ceil(float64(total) / float64(perPage)))

	// 4. Fetch the page.
	offset := (page - 1) * perPage
	var clients []models.Client
	if err := q.Order("name ASC").Limit(perPage).Offset(offset).Find(&clients).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_to_list_clients"})
		return
	}

	// 5. Batch-load active subscriptions for this page (single query instead of N+1).
	premiumSet := make(map[uint]bool)
	if len(clients) > 0 {
		clientIDs := make([]uint, len(clients))
		for i, cl := range clients {
			clientIDs[i] = cl.ID
		}
		now := time.Now().UTC()
		var premiumIDs []uint
		h.db.WithContext(ctx).
			Table("subscriptions").
			Select("client_id").
			Where("barbershop_id = ? AND client_id IN ? AND status = 'active' AND current_period_start <= ? AND current_period_end > ?",
				barbershopID, clientIDs, now, now).
			Scan(&premiumIDs)
		for _, id := range premiumIDs {
			premiumSet[id] = true
		}
	}

	out := make([]ClientListItemResponse, 0, len(clients))
	for _, client := range clients {
		out = append(out, ClientListItemResponse{
			Client:   client,
			Category: categoryByClient[client.ID],
			Premium:  premiumSet[client.ID],
		})
	}

	c.JSON(http.StatusOK, ClientListResponse{
		Clients:        out,
		Total:          total,
		Page:           page,
		PerPage:        perPage,
		TotalPages:     totalPages,
		CategoryCounts: categoryCounts,
	})
}
