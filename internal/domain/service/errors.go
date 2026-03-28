package service

import "errors"

var (
	ErrInvalidContext  = errors.New("invalid_context")
	ErrServiceNotFound = errors.New("service_not_found")
	ErrInvalidName     = errors.New("invalid_name")
	ErrInvalidDuration = errors.New("invalid_duration")
	ErrInvalidPrice    = errors.New("invalid_price")
)
