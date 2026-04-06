package payment

import (
	"context"
	"fmt"
	"strconv"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/payment"
	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

const mpPrefPrefix = "mp_pref:"

// CreateMPPreference cria uma preferência de pagamento no Mercado Pago para um agendamento.
// É idempotente: se o pagamento já tiver uma preferência gerada, reutiliza sem chamar a API novamente.
type CreateMPPreference struct {
	repo    domain.Repository
	mp      domain.MPGateway
	audit   *audit.Dispatcher
	appURL  string // URL base do frontend, ex: https://app.seudominio.com
	backURL string // URL base do backend, ex: https://api.seudominio.com
}

func NewCreateMPPreference(
	repo domain.Repository,
	mp domain.MPGateway,
	audit *audit.Dispatcher,
	appURL string,
	backURL string,
) *CreateMPPreference {
	return &CreateMPPreference{
		repo:    repo,
		mp:      mp,
		audit:   audit,
		appURL:  appURL,
		backURL: backURL,
	}
}

func (uc *CreateMPPreference) Execute(
	ctx context.Context,
	barbershopID uint,
	appointmentID uint,
	slug string,
) (*models.Payment, *domain.MPPreference, error) {

	// ==================================================
	// 1) BEGIN TX
	// ==================================================
	tx, err := uc.repo.BeginTx(ctx, barbershopID)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()

	// ==================================================
	// 2) Lock do payment por appointment (FOR UPDATE)
	// ==================================================
	payment, err := tx.GetByAppointmentIDForUpdate(ctx, barbershopID, appointmentID)
	if err != nil {
		return nil, nil, err
	}
	if payment == nil {
		return nil, nil, httperr.ErrBusiness("payment_not_found")
	}

	// Idempotente: já existe uma preferência MP
	if payment.TxID != nil && len(*payment.TxID) > len(mpPrefPrefix) {
		txid := *payment.TxID
		if len(txid) > len(mpPrefPrefix) && txid[:len(mpPrefPrefix)] == mpPrefPrefix {
			prefID := txid[len(mpPrefPrefix):]
			initPoint := ""
			if payment.QRCode != nil {
				initPoint = *payment.QRCode
			}
			if err := tx.Commit(); err != nil {
				return nil, nil, err
			}
			return payment, &domain.MPPreference{
				PreferenceID: prefID,
				InitPoint:    initPoint,
			}, nil
		}
	}

	if domain.Status(payment.Status) != domain.StatusPending {
		return nil, nil, httperr.ErrBusiness("payment_not_pending")
	}

	if payment.Amount <= 100 {
		return nil, nil, httperr.ErrBusiness("invalid_amount")
	}

	// ==================================================
	// 3) Montar back_urls e notification_url
	// ==================================================
	backURLs := domain.MPBackURLs{
		Success: fmt.Sprintf("%s/%s/checkout/pagamento/mp/sucesso", uc.appURL, slug),
		Pending: fmt.Sprintf("%s/%s/checkout/pagamento/mp/pendente", uc.appURL, slug),
		Failure: fmt.Sprintf("%s/%s/checkout/pagamento/mp/erro", uc.appURL, slug),
	}
	notificationURL := fmt.Sprintf("%s/api/webhooks/mp", uc.backURL)
	externalReference := strconv.FormatUint(uint64(payment.ID), 10)
	description := fmt.Sprintf("Agendamento #%d", appointmentID)

	// ==================================================
	// 4) Criar preferência no Mercado Pago
	// ==================================================
	pref, err := uc.mp.CreatePreference(
		payment.Amount,
		description,
		externalReference,
		notificationURL,
		backURLs,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("mp preference creation failed: %w", err)
	}

	// ==================================================
	// 5) Persistir preferência como TxID e init_point no QRCode
	// ==================================================
	txid := mpPrefPrefix + pref.PreferenceID
	payment.TxID = &txid
	payment.QRCode = &pref.InitPoint

	if err := tx.UpdatePaymentTx(ctx, barbershopID, payment); err != nil {
		return nil, nil, fmt.Errorf("failed updating payment: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("commit failed: %w", err)
	}

	uc.audit.Dispatch(audit.Event{
		BarbershopID: barbershopID,
		Action:       "payment_mp_preference_created",
		Entity:       "payment",
		EntityID:     &payment.ID,
		Metadata: map[string]any{
			"preference_id": pref.PreferenceID,
		},
	})

	return payment, pref, nil
}
