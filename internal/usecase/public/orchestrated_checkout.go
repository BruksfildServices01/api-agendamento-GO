package public

import (
	"context"
	"fmt"
	"strings"

	orderDomain "github.com/BruksfildServices01/barber-scheduler/internal/domain/order"
	domainService "github.com/BruksfildServices01/barber-scheduler/internal/domain/service"
	"github.com/BruksfildServices01/barber-scheduler/internal/dto"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
	ucAppointment "github.com/BruksfildServices01/barber-scheduler/internal/usecase/appointment"
	ucCart "github.com/BruksfildServices01/barber-scheduler/internal/usecase/cart"
)

type OrchestratedCheckout struct {
	createAppointmentUC *ucAppointment.CreatePrivateAppointment
	getCartUC           *ucCart.GetCart
	checkoutCartUC      *ucCart.CheckoutCart
	serviceRepo         domainService.Repository
}

func NewOrchestratedCheckout(
	createAppointmentUC *ucAppointment.CreatePrivateAppointment,
	getCartUC *ucCart.GetCart,
	checkoutCartUC *ucCart.CheckoutCart,
	serviceRepo domainService.Repository,
) *OrchestratedCheckout {
	return &OrchestratedCheckout{
		createAppointmentUC: createAppointmentUC,
		getCartUC:           getCartUC,
		checkoutCartUC:      checkoutCartUC,
		serviceRepo:         serviceRepo,
	}
}

func (uc *OrchestratedCheckout) Execute(
	ctx context.Context,
	barbershopID uint,
	input dto.PublicOrchestratedCheckoutRequestDTO,
) (*dto.PublicOrchestratedCheckoutResponseDTO, error) {

	service, err := uc.serviceRepo.GetByID(ctx, barbershopID, input.ServiceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get service: %w", err)
	}
	if service == nil {
		return nil, domainService.ErrServiceNotFound
	}

	appointment, err := uc.createAppointmentUC.Execute(
		ctx,
		ucAppointment.CreatePrivateAppointmentInput{
			BarbershopID:   barbershopID,
			BarberID:       input.BarberID,
			ClientName:     input.ClientName,
			ClientPhone:    input.ClientPhone,
			ClientEmail:    input.ClientEmail,
			ProductID:      input.ServiceID,
			Date:           input.Date,
			Time:           input.Time,
			Notes:          input.Notes,
			IdempotencyKey: input.IdempotencyKey,
		},
	)
	if err != nil {
		return nil, err
	}

	var orderDTO *dto.PublicOrchestratedCheckoutOrderDTO
	var order *orderDomain.Order
	var productsAmountCents int64

	cartKey := ""
	if input.CartKey != nil {
		cartKey = strings.TrimSpace(*input.CartKey)
	}

	if cartKey != "" && uc.getCartUC != nil {
		cartView, err := uc.getCartUC.Execute(
			ctx,
			ucCart.GetCartInput{
				CartKey:      cartKey,
				BarbershopID: barbershopID,
			},
		)
		if err != nil {
			return nil, err
		}

		if cartView != nil && len(cartView.Items) > 0 {
			order, err = uc.checkoutCartUC.Execute(
				ctx,
				ucCart.CheckoutCartInput{
					CartKey:      cartKey,
					BarbershopID: barbershopID,
				},
			)
			if err != nil {
				return nil, err
			}

			orderDTO = &dto.PublicOrchestratedCheckoutOrderDTO{
				ID:         order.ID,
				Status:     string(order.Status),
				TotalCents: order.TotalAmount,
				ItemsCount: len(order.Items),
			}
			productsAmountCents = order.TotalAmount
		}
	}

	serviceAmountCents := service.Price
	totalAmountCents := serviceAmountCents + productsAmountCents

	appointmentPaymentRequired := appointment.Status == models.AppointmentStatusAwaitingPayment
	orderPaymentRequired := order != nil && order.Status == orderDomain.OrderStatusPending
	multiplePaymentsRequired := appointmentPaymentRequired && orderPaymentRequired

	warning := ""
	if multiplePaymentsRequired {
		warning = "Existem dois pagamentos PIX pendentes: um do agendamento e outro do pedido."
	}

	response := &dto.PublicOrchestratedCheckoutResponseDTO{
		Appointment: &dto.PublicOrchestratedCheckoutAppointmentDTO{
			ID:                 appointment.ID,
			Status:             string(appointment.Status),
			StartTime:          appointment.StartTime,
			EndTime:            appointment.EndTime,
			ServiceID:          service.ID,
			ServiceName:        service.Name,
			ServiceAmountCents: service.Price,
		},
		Order: orderDTO,
		Summary: dto.PublicOrchestratedCheckoutSummaryDTO{
			ServiceAmountCents:  serviceAmountCents,
			ProductsAmountCents: productsAmountCents,
			TotalAmountCents:    totalAmountCents,
		},
		Payments: dto.PublicOrchestratedCheckoutPaymentsDTO{
			AppointmentPaymentRequired: appointmentPaymentRequired,
			OrderPaymentRequired:       orderPaymentRequired,
			MultiplePaymentsRequired:   multiplePaymentsRequired,
		},
		NextStep: buildNextStep(appointmentPaymentRequired, orderPaymentRequired),
		NextURLs: dto.PublicOrchestratedCheckoutURLsDTO{},
		Warning:  warning,
	}

	return response, nil
}

func buildNextStep(appointmentPaymentRequired, orderPaymentRequired bool) dto.PublicOrchestratedCheckoutNextStepDTO {
	switch {
	case appointmentPaymentRequired && orderPaymentRequired:
		return dto.PublicOrchestratedCheckoutNextStepDTO{
			Action:   "multiple_payments_required",
			Method:   "pix",
			Guidance: "O agendamento e o pedido foram criados e ambos possuem pagamento PIX pendente.",
		}

	case appointmentPaymentRequired:
		return dto.PublicOrchestratedCheckoutNextStepDTO{
			Action:   "appointment_payment_required",
			Method:   "pix",
			Guidance: "O agendamento foi criado e exige pagamento PIX para confirmação.",
		}

	case orderPaymentRequired:
		return dto.PublicOrchestratedCheckoutNextStepDTO{
			Action:   "order_payment_required",
			Method:   "pix",
			Guidance: "O pedido foi criado e está pendente de pagamento PIX.",
		}

	default:
		return dto.PublicOrchestratedCheckoutNextStepDTO{
			Action:   "completed",
			Guidance: "O checkout foi concluído com sucesso.",
		}
	}
}
