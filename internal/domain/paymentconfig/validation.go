package paymentconfig

import "github.com/BruksfildServices01/barber-scheduler/internal/httperr"

// ======================================================
// CONFIG VALIDATION
// ======================================================

// Validate valida as invariantes da configuração principal
func Validate(c *Config) error {
	if c.PixExpirationMinutes <= 0 {
		return httperr.ErrBusiness("invalid_pix_expiration")
	}

	switch c.DefaultRequirement {
	case PaymentMandatory, PaymentOptional, PaymentNone:
		// ok
	default:
		return httperr.ErrBusiness("invalid_payment_requirement")
	}

	return nil
}

func IsValidRequirement(r PaymentRequirement) bool {
	switch r {
	case PaymentMandatory, PaymentOptional, PaymentNone:
		return true
	default:
		return false
	}
}
