package handlers

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/infra/storage"
	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

const maxImageBytes = 5 << 20 // 5 MB

var allowedMIME = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/webp": "webp",
}

type ImageHandler struct {
	db  *gorm.DB
	r2  *storage.R2Service
}

func NewImageHandler(db *gorm.DB, r2 *storage.R2Service) *ImageHandler {
	return &ImageHandler{db: db, r2: r2}
}

// ──────────────────────────────────────────────
// SERVICE IMAGES  (up to 3 per service)
// ──────────────────────────────────────────────

// POST /api/me/services/:id/images
func (h *ImageHandler) AddServiceImage(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	serviceID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_service_id"})
		return
	}

	// Verify service belongs to barbershop.
	var svc models.BarbershopService
	if err := h.db.Where("id = ? AND barbershop_id = ?", serviceID, barbershopID).First(&svc).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "service_not_found"})
		return
	}

	// Check current image count.
	var count int64
	h.db.Model(&models.ServiceImage{}).Where("barbershop_service_id = ?", serviceID).Count(&count)
	if count >= 3 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "max_images_reached"})
		return
	}

	raw, mime, err := readImageFromRequest(c, "image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_ = mime

	key := fmt.Sprintf("%d/services/%s.jpg", barbershopID, uuid.NewString())
	url, err := h.r2.Upload(c.Request.Context(), storage.KindServiceImage, key, raw)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "upload_failed"})
		return
	}

	img := models.ServiceImage{
		BarbershopServiceID: uint(serviceID),
		URL:                 url,
		Position:            int(count),
	}
	if err := h.db.Create(&img).Error; err != nil {
		_ = h.r2.Delete(c.Request.Context(), key)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db_error"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": img.ID, "url": img.URL, "position": img.Position})
}

// DELETE /api/me/services/:id/images/:imageId
func (h *ImageHandler) DeleteServiceImage(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	serviceID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_service_id"})
		return
	}

	imageID, err := strconv.ParseUint(c.Param("imageId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_image_id"})
		return
	}

	// Load image and verify ownership via join.
	var img models.ServiceImage
	err = h.db.
		Joins("JOIN barbershop_services s ON s.id = service_images.barbershop_service_id").
		Where("service_images.id = ? AND service_images.barbershop_service_id = ? AND s.barbershop_id = ?",
			imageID, serviceID, barbershopID).
		First(&img).Error
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "image_not_found"})
		return
	}

	key := h.r2.KeyFromURL(img.URL)
	_ = h.r2.Delete(c.Request.Context(), key)

	h.db.Delete(&img)

	// Re-sequence positions.
	var remaining []models.ServiceImage
	h.db.Where("barbershop_service_id = ?", serviceID).Order("position ASC").Find(&remaining)
	for i, rem := range remaining {
		h.db.Model(&rem).Update("position", i)
	}

	c.Status(http.StatusNoContent)
}

// ──────────────────────────────────────────────
// PRODUCT IMAGE  (1 per product)
// ──────────────────────────────────────────────

// PUT /api/me/products/:id/image
func (h *ImageHandler) SetProductImage(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	productID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_product_id"})
		return
	}

	var product models.Product
	if err := h.db.Where("id = ? AND barbershop_id = ?", productID, barbershopID).First(&product).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "product_not_found"})
		return
	}

	// Delete previous image from R2 if exists.
	if product.ImageURL != nil && *product.ImageURL != "" {
		_ = h.r2.Delete(c.Request.Context(), h.r2.KeyFromURL(*product.ImageURL))
	}

	raw, _, err := readImageFromRequest(c, "image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	key := fmt.Sprintf("%d/products/%s.jpg", barbershopID, uuid.NewString())
	url, err := h.r2.Upload(c.Request.Context(), storage.KindProductImage, key, raw)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "upload_failed"})
		return
	}

	if err := h.db.Model(&product).Update("image_url", url).Error; err != nil {
		_ = h.r2.Delete(c.Request.Context(), key)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db_error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": url})
}

// DELETE /api/me/products/:id/image
func (h *ImageHandler) DeleteProductImage(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	productID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_product_id"})
		return
	}

	var product models.Product
	if err := h.db.Where("id = ? AND barbershop_id = ?", productID, barbershopID).First(&product).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "product_not_found"})
		return
	}

	if product.ImageURL != nil && *product.ImageURL != "" {
		_ = h.r2.Delete(c.Request.Context(), h.r2.KeyFromURL(*product.ImageURL))
	}

	h.db.Model(&product).Update("image_url", nil)
	c.Status(http.StatusNoContent)
}

// ──────────────────────────────────────────────
// BARBERSHOP PROFILE PHOTO  (1)
// ──────────────────────────────────────────────

// PUT /api/me/profile/photo
func (h *ImageHandler) SetProfilePhoto(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	var shop models.Barbershop
	if err := h.db.Where("id = ?", barbershopID).First(&shop).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "barbershop_not_found"})
		return
	}

	if shop.PhotoURL != nil && *shop.PhotoURL != "" {
		_ = h.r2.Delete(c.Request.Context(), h.r2.KeyFromURL(*shop.PhotoURL))
	}

	raw, _, err := readImageFromRequest(c, "photo")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	key := fmt.Sprintf("%d/profile/%s.jpg", barbershopID, uuid.NewString())
	url, err := h.r2.Upload(c.Request.Context(), storage.KindProfilePhoto, key, raw)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "upload_failed"})
		return
	}

	if err := h.db.Model(&shop).Update("photo_url", url).Error; err != nil {
		_ = h.r2.Delete(c.Request.Context(), key)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db_error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": url})
}

// DELETE /api/me/profile/photo
func (h *ImageHandler) DeleteProfilePhoto(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	var shop models.Barbershop
	if err := h.db.Where("id = ?", barbershopID).First(&shop).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "barbershop_not_found"})
		return
	}

	if shop.PhotoURL != nil && *shop.PhotoURL != "" {
		_ = h.r2.Delete(c.Request.Context(), h.r2.KeyFromURL(*shop.PhotoURL))
		h.db.Model(&shop).Update("photo_url", nil)
	}

	c.Status(http.StatusNoContent)
}

// ──────────────────────────────────────────────
// Shared helper
// ──────────────────────────────────────────────

func readImageFromRequest(c *gin.Context, field string) ([]byte, string, error) {
	file, header, err := c.Request.FormFile(field)
	if err != nil {
		return nil, "", fmt.Errorf("missing_file")
	}
	defer file.Close()

	if header.Size > maxImageBytes {
		return nil, "", fmt.Errorf("file_too_large")
	}

	mime := header.Header.Get("Content-Type")
	// Normalize; browser sometimes sends "image/jpg"
	mime = strings.ToLower(strings.Split(mime, ";")[0])
	if mime == "image/jpg" {
		mime = "image/jpeg"
	}

	if _, ok := allowedMIME[mime]; !ok {
		return nil, "", fmt.Errorf("unsupported_format")
	}

	raw, err := io.ReadAll(io.LimitReader(file, maxImageBytes))
	if err != nil {
		return nil, "", fmt.Errorf("read_error")
	}

	return raw, mime, nil
}
