package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AppWebHandler struct {
	db *gorm.DB
}

func NewAppWebHandler(db *gorm.DB) *AppWebHandler {
	return &AppWebHandler{db: db}
}

func (h *AppWebHandler) LoginPage(c *gin.Context) {
	c.HTML(http.StatusOK, "base", gin.H{
		"Page": "login",
	})
}

func (h *AppWebHandler) Dashboard(c *gin.Context) {
	c.HTML(http.StatusOK, "base", gin.H{
		"Page": "dashboard",
	})
}

func (h *AppWebHandler) Services(c *gin.Context) {
	c.HTML(http.StatusOK, "base", gin.H{
		"Page": "services",
	})
}
