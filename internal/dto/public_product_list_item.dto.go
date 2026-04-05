package dto

type PublicProductListItemDTO struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	PriceCents  int64  `json:"price_cents"`
	ImageURL    string `json:"image_url,omitempty"`
	Category    string `json:"category,omitempty"`
}
