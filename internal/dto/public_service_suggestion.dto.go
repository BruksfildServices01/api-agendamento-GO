package dto

type PublicSuggestedProductDTO struct {
	ProductID   uint   `json:"product_id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category,omitempty"`
	PriceCents  int64  `json:"price_cents"`
	ImageURL    string `json:"image_url,omitempty"`
}

type PublicServiceSuggestionDTO struct {
	ServiceID uint                       `json:"service_id"`
	Product   *PublicSuggestedProductDTO `json:"product,omitempty"`
}
