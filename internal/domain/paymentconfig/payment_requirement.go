package paymentconfig

type PaymentRequirement string

const (
	PaymentMandatory PaymentRequirement = "mandatory"
	PaymentOptional  PaymentRequirement = "optional"
	PaymentNone      PaymentRequirement = "none"
)
