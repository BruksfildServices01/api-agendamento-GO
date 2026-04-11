package public

import (
	"context"
	"fmt"
	"log"
	"strings"

	"gorm.io/gorm"

	orderDomain "github.com/BruksfildServices01/barber-scheduler/internal/domain/order"
	domainNotification "github.com/BruksfildServices01/barber-scheduler/internal/domain/notification"
	domainService "github.com/BruksfildServices01/barber-scheduler/internal/domain/service"
	"github.com/BruksfildServices01/barber-scheduler/internal/dto"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
	ucAppointment   "github.com/BruksfildServices01/barber-scheduler/internal/usecase/appointment"
	ucCart          "github.com/BruksfildServices01/barber-scheduler/internal/usecase/cart"
	ucSuggestion    "github.com/BruksfildServices01/barber-scheduler/internal/usecase/servicesuggestion"
	ucTicket        "github.com/BruksfildServices01/barber-scheduler/internal/usecase/ticket"
)

type OrchestratedCheckout struct {
	createAppointmentUC *ucAppointment.CreatePrivateAppointment
	getCartUC           *ucCart.GetCart
	checkoutCartUC      *ucCart.CheckoutCart
	serviceRepo         domainService.Repository
	generateTicketUC    *ucTicket.GenerateTicket
	getSuggestionUC     *ucSuggestion.GetPublicServiceSuggestion
	db                  *gorm.DB
	apptNotifier        domainNotification.AppointmentNotifier
	appURL              string
}

func NewOrchestratedCheckout(
	createAppointmentUC *ucAppointment.CreatePrivateAppointment,
	getCartUC *ucCart.GetCart,
	checkoutCartUC *ucCart.CheckoutCart,
	serviceRepo domainService.Repository,
	generateTicketUC *ucTicket.GenerateTicket,
	db *gorm.DB,
	apptNotifier domainNotification.AppointmentNotifier,
	appURL string,
	getSuggestionUC *ucSuggestion.GetPublicServiceSuggestion,
) *OrchestratedCheckout {
	return &OrchestratedCheckout{
		createAppointmentUC: createAppointmentUC,
		getCartUC:           getCartUC,
		checkoutCartUC:      checkoutCartUC,
		serviceRepo:         serviceRepo,
		generateTicketUC:    generateTicketUC,
		getSuggestionUC:     getSuggestionUC,
		db:                  db,
		apptNotifier:        apptNotifier,
		appURL:              appURL,
	}
}

func (uc *OrchestratedCheckout) Execute(
	ctx context.Context,
	barbershopID uint,
	input dto.PublicOrchestratedCheckoutRequestDTO,
) (*dto.PublicOrchestratedCheckoutResponseDTO, error) {

	var barber struct {
		ID uint `gorm:"column:id"`
	}
	if err := uc.db.WithContext(ctx).
		Raw("SELECT id FROM users WHERE barbershop_id = ? AND role = 'owner' LIMIT 1", barbershopID).
		Scan(&barber).Error; err != nil || barber.ID == 0 {
		return nil, fmt.Errorf("barber not found for barbershop %d", barbershopID)
	}

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
			BarberID:       barber.ID,
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

	var ticketToken string
	if uc.generateTicketUC != nil {
		ticketToken, err = uc.generateTicketUC.Execute(ctx, ucTicket.GenerateTicketInput{
			AppointmentID: appointment.ID,
			BarbershopID:  barbershopID,
			StartTime:     appointment.StartTime,
		})
		if err != nil {
			log.Printf("[OrchestratedCheckout] failed to generate ticket for appointment %d: %v", appointment.ID, err)
			ticketToken = ""
		}
	}

	// Fire async appointment confirmation notification
	// Only notify after confirmed payment — skip if appointment is awaiting payment.
	if uc.apptNotifier != nil && input.ClientEmail != "" &&
		appointment.Status != models.AppointmentStatusAwaitingPayment {
		type bsRow struct {
			Name     string `gorm:"column:name"`
			Phone    string `gorm:"column:phone"`
			Timezone string `gorm:"column:timezone"`
		}
		var bs bsRow
		if dbErr := uc.db.WithContext(ctx).
			Raw("SELECT name, phone, timezone FROM barbershops WHERE id = ?", barbershopID).
			Scan(&bs).Error; dbErr != nil {
			log.Printf("[OrchestratedCheckout] failed to query barbershop for notification: %v", dbErr)
		} else {
			ticketURL := ""
			if ticketToken != "" {
				ticketURL = uc.appURL + "/ticket/" + ticketToken
			}
			notifyInput := domainNotification.AppointmentConfirmedInput{
				ClientName:      input.ClientName,
				ClientEmail:     input.ClientEmail,
				BarbershopName:  bs.Name,
				BarbershopPhone: bs.Phone,
				ServiceName:     service.Name,
				StartTime:       appointment.StartTime,
				EndTime:         appointment.EndTime,
				Timezone:        bs.Timezone,
				TicketURL:       ticketURL,
			}
			_ = uc.apptNotifier.NotifyConfirmed(ctx, notifyInput)
		}
	}

	var orderDTO *dto.PublicOrchestratedCheckoutOrderDTO
	var order *orderDomain.Order
	var productsAmountCents int64

	cartKey := ""
	if input.CartKey != nil {
		cartKey = strings.TrimSpace(*input.CartKey)
	}

	if cartKey != "" && uc.getCartUC != nil && appointment.Status == models.AppointmentStatusAwaitingPayment {
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

	// Fetch service suggestion non-fatally — a failure here must not abort the checkout.
	var suggestionDTO *dto.PublicOrchestratedCheckoutSuggestionDTO
	if uc.getSuggestionUC != nil {
		sugg, suggErr := uc.getSuggestionUC.Execute(ctx, ucSuggestion.GetPublicServiceSuggestionInput{
			BarbershopID: barbershopID,
			ServiceID:    service.ID,
		})
		if suggErr != nil {
			log.Printf("[OrchestratedCheckout] suggestion fetch failed for service %d: %v", service.ID, suggErr)
		} else if sugg != nil && sugg.Product != nil {
			suggestionDTO = &dto.PublicOrchestratedCheckoutSuggestionDTO{
				ProductID:   sugg.Product.ID,
				Name:        sugg.Product.Name,
				Description: sugg.Product.Description,
				Category:    sugg.Product.Category,
				PriceCents:  sugg.Product.Price,
				ImageURL:    sugg.Product.ImageURL,
			}
		}
	}

	serviceAmountCents := service.Price
	totalAmountCents := serviceAmountCents + productsAmountCents

	appointmentPaymentRequired := appointment.Status == models.AppointmentStatusAwaitingPayment
	orderPaymentRequired := order != nil && order.Status == orderDomain.OrderStatusPending
	multiplePaymentsRequired := appointmentPaymentRequired && orderPaymentRequired

	warning := ""
	if multiplePaymentsRequired {
		warning = "Existem dois pagamentos pendentes: um do agendamento e outro do pedido."
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
		NextStep:   buildNextStep(appointmentPaymentRequired, orderPaymentRequired),
		NextURLs:   dto.PublicOrchestratedCheckoutURLsDTO{},
		Suggestion: suggestionDTO,
		Warning:    warning,
	}

	if ticketToken != "" {
		response.NextURLs.TicketURL = "/api/public/ticket/" + ticketToken // API path for frontend routing
	}

	return response, nil
}

func buildNextStep(appointmentPaymentRequired, orderPaymentRequired bool) dto.PublicOrchestratedCheckoutNextStepDTO {
	switch {
	case appointmentPaymentRequired && orderPaymentRequired:
		return dto.PublicOrchestratedCheckoutNextStepDTO{
			Action:   "multiple_payments_required",
			Method:   "mp",
			Guidance: "O agendamento e o pedido foram criados e ambos possuem pagamento pendente.",
		}

	case appointmentPaymentRequired:
		return dto.PublicOrchestratedCheckoutNextStepDTO{
			Action:   "appointment_payment_required",
			Method:   "mp",
			Guidance: "O agendamento foi criado e exige pagamento para confirmação.",
		}

	case orderPaymentRequired:
		return dto.PublicOrchestratedCheckoutNextStepDTO{
			Action:   "order_payment_required",
			Method:   "mp",
			Guidance: "O pedido foi criado e está pendente de pagamento.",
		}

	default:
		return dto.PublicOrchestratedCheckoutNextStepDTO{
			Action:   "completed",
			Guidance: "O checkout foi concluído com sucesso.",
		}
	}
}
