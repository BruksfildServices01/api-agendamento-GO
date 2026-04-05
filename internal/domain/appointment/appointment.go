package appointment

type AppointmentCreator string

const (
	CreatedByBarber AppointmentCreator = "barber"
	CreatedByClient AppointmentCreator = "client"
)

type PaymentIntent string

const (
	PaymentPaid     PaymentIntent = "paid"
	PaymentPayLater PaymentIntent = "pay_later"
)
