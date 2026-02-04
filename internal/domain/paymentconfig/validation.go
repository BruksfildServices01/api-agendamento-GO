package paymentconfig

import "github.com/BruksfildServices01/barber-scheduler/internal/httperr"

func Validate(c *Config) error {
	if c.PixExpirationMinutes <= 0 {
		return httperr.ErrBusiness("invalid_pix_expiration")
	}

	return nil
}
