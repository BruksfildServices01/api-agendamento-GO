package models

import "time"

type Payment struct {
	ID            uint          `gorm:"primaryKey"`
	BarbershopID  uint          `gorm:"index;not null"`
	Barbershop    *Barbershop   `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	AppointmentID *uint         `gorm:"index"`
	Appointment   *Appointment  `gorm:"constraint:OnDelete:CASCADE;"`
	OrderID       *uint         `gorm:"index"`
	Order         *Order        `gorm:"constraint:OnDelete:CASCADE;"`
	// BundledOrderID: quando um pagamento de agendamento também cobre um pedido de produtos,
	// o ID do pedido é armazenado aqui (sem violar a constraint payment_exactly_one_target).
	BundledOrderID *uint         `gorm:"column:bundled_order_id;index"`
	SubscriptionID *uint         `gorm:"index"`
	Subscription   *Subscription `gorm:"constraint:OnDelete:SET NULL;"`
	TxID              *string `gorm:"column:txid;size:100;uniqueIndex"`
	MPPaymentID       *int64  `gorm:"column:mp_payment_id;index"`
	// Provider identifica o gateway que criou este pagamento ("mercadopago", "pagbank").
	// Usado no polling de status para consultar o provider correto independentemente
	// de qual provider está atualmente ativo na barbearia.
	Provider          *string `gorm:"column:provider;size:50"`
	// ProviderPaymentID é o ID externo puro do provider — sem prefixos internos como "mp_pay:".
	// Ex: "123456" (MP), "QRC_XXXXX" (PagBank PIX), "CHAR_XXXXX" (PagBank cartão).
	ProviderPaymentID *string `gorm:"column:provider_payment_id;size:100"`
	QRCode            *string `gorm:"type:text"`
	Amount        int64         `gorm:"type:bigint;not null"`
	Status        PaymentStatus `gorm:"type:payment_status;not null"`
	PaidAt        *time.Time
	ExpiresAt     *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
