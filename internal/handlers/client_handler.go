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

func (h *ClientHandler) List(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)
	ctx := c.Request.Context()

	query := strings.ToLower(strings.TrimSpace(c.Query("q")))
	if query == "" {
		query = strings.ToLower(strings.TrimSpace(c.Query("query")))
	}
	categoryFilter := c.Query("category")
	premiumFilter := c.Query("premium") == "true"

	page := 1
	if p, err := strconv.Atoi(c.Query("page")); err == nil && p > 0 {
		page = p
	}
	perPage := 30
	if pp, err := strconv.Atoi(c.Query("per_page")); err == nil && pp > 0 && pp <= 100 {
		perPage = pp
	}

	// 1. category_counts via aggregation — uma query, sem carregar todos os registros.
	type countRow struct {
		Category string `gorm:"column:category"`
		Count    int    `gorm:"column:count"`
	}
	var countRows []countRow
	h.db.WithContext(ctx).Raw(`
		SELECT category, COUNT(*) AS count
		FROM client_metrics
		WHERE barbershop_id = ?
		GROUP BY category
	`, barbershopID).Scan(&countRows)

	categoryCounts := map[string]int{
		"at_risk": 0, "new": 0, "trusted": 0, "regular": 0, "premium": 0,
	}
	for _, r := range countRows {
		categoryCounts[r.Category] = r.Count
	}

	now := time.Now().UTC()
	var premiumCount int64
	h.db.WithContext(ctx).Table("subscriptions").
		Where("barbershop_id = ? AND status = 'active' AND current_period_start <= ? AND current_period_end > ?",
			barbershopID, now, now).
		Count(&premiumCount)
	categoryCounts["premium"] = int(premiumCount)

	// 2. Base query — filtros via subquery, sem carregar IDs em memória.
	q := h.db.WithContext(ctx).Model(&models.Client{}).Where("barbershop_id = ?", barbershopID)

	if query != "" {
		like := "%" + query + "%"
		q = q.Where("LOWER(name) LIKE ? OR phone LIKE ? OR LOWER(email) LIKE ?", like, like, like)
	}

	if premiumFilter {
		q = q.Where(`id IN (
			SELECT client_id FROM subscriptions
			WHERE barbershop_id = ? AND status = 'active'
			  AND current_period_start <= ? AND current_period_end > ?
		)`, barbershopID, now, now)
	} else if categoryFilter != "" {
		q = q.Where(`id IN (
			SELECT client_id FROM client_metrics
			WHERE barbershop_id = ? AND category = ?
		)`, barbershopID, categoryFilter)
	}

	// 3. Count.
	var total int64
	if err := q.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_to_count_clients"})
		return
	}
	totalPages := int(math.Ceil(float64(total) / float64(perPage)))

	// 4. Busca a página.
	offset := (page - 1) * perPage
	var clients []models.Client
	if err := q.Order("name ASC").Limit(perPage).Offset(offset).Find(&clients).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_to_list_clients"})
		return
	}

	if len(clients) == 0 {
		c.JSON(http.StatusOK, ClientListResponse{
			Clients: []ClientListItemResponse{}, Total: total, Page: page,
			PerPage: perPage, TotalPages: totalPages, CategoryCounts: categoryCounts,
		})
		return
	}

	// 5. Carrega category e premium APENAS para os IDs desta página.
	pageIDs := make([]uint, len(clients))
	for i, cl := range clients {
		pageIDs[i] = cl.ID
	}

	type metricRow struct {
		ClientID uint   `gorm:"column:client_id"`
		Category string `gorm:"column:category"`
	}
	var metricRows []metricRow
	h.db.WithContext(ctx).Raw(`
		SELECT client_id, category FROM client_metrics
		WHERE barbershop_id = ? AND client_id IN ?
	`, barbershopID, pageIDs).Scan(&metricRows)

	categoryByClient := make(map[uint]string, len(metricRows))
	for _, r := range metricRows {
		categoryByClient[r.ClientID] = r.Category
	}

	var pageActiveSubs []uint
	h.db.WithContext(ctx).Table("subscriptions").Select("client_id").
		Where("barbershop_id = ? AND client_id IN ? AND status = 'active' AND current_period_start <= ? AND current_period_end > ?",
			barbershopID, pageIDs, now, now).
		Scan(&pageActiveSubs)
	premiumSet := make(map[uint]bool, len(pageActiveSubs))
	for _, id := range pageActiveSubs {
		premiumSet[id] = true
	}

	// 6. Monta resposta.
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
