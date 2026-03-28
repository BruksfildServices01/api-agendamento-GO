package servicesuggestion

import "errors"

var (
	ErrInvalidContext          = errors.New("invalid_context")
	ErrServiceNotFound         = errors.New("service_not_found")
	ErrProductNotFound         = errors.New("product_not_found")
	ErrInvalidSuggestedProduct = errors.New("invalid_suggested_product")
	ErrSuggestionNotFound      = errors.New("suggestion_not_found")
)
