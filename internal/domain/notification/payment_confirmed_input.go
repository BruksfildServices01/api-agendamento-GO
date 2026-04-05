package notification

import "time"

type PaymentConfirmedInput struct {
	PaymentID uint

	BarbershopName    string
	BarbershopSlug    string
	BarbershopAddress string
	BarbershopPhone   string

	ClientName  string
	ClientEmail string

	ServiceName string

	StartTime time.Time
	EndTime   time.Time
	Timezone  string

	PublicURL string
}
