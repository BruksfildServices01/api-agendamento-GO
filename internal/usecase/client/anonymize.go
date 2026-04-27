package client

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

var (
	ErrClientNotFound         = errors.New("client_not_found")
	ErrAlreadyAnonymized      = errors.New("already_anonymized")
	ErrActiveSubscription     = errors.New("active_subscription_exists")
	ErrFutureAppointments     = errors.New("future_appointments_exist")
)

// AnonymizeClient remove dados pessoais de um cliente em transação atômica.
// O registro do cliente é mantido para preservar histórico financeiro e operacional.
// Após anonimização: name="Cliente removido", phone=NULL, email=NULL.
type AnonymizeClient struct {
	db    *gorm.DB
	audit *audit.Dispatcher
}

func NewAnonymizeClient(db *gorm.DB, auditDispatcher *audit.Dispatcher) *AnonymizeClient {
	return &AnonymizeClient{db: db, audit: auditDispatcher}
}

func (uc *AnonymizeClient) Execute(
	ctx context.Context,
	barbershopID uint,
	clientID uint,
	userID uint,
	reason string,
) error {
	if reason == "" {
		reason = "lgpd_request"
	}

	return uc.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {

		// 1. Buscar cliente validando ownership por barbershop_id
		var client models.Client
		if err := tx.Where("id = ? AND barbershop_id = ?", clientID, barbershopID).
			First(&client).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return httperr.ErrBusiness("client_not_found")
			}
			return err
		}

		// 2. Já anonimizado?
		if client.AnonymizedAt != nil {
			return httperr.ErrBusiness("already_anonymized")
		}

		// 3. Subscription ativa bloqueante
		var activeSubCount int64
		if err := tx.Model(&models.Subscription{}).
			Where("barbershop_id = ? AND client_id = ? AND status = 'active'",
				barbershopID, clientID).
			Count(&activeSubCount).Error; err != nil {
			return err
		}
		if activeSubCount > 0 {
			return httperr.ErrBusiness("active_subscription_exists")
		}

		// 4. Agendamentos futuros ativos
		var futureApptCount int64
		if err := tx.Model(&models.Appointment{}).
			Where("barbershop_id = ? AND client_id = ? AND status IN ('scheduled','awaiting_payment') AND start_time > ?",
				barbershopID, clientID, time.Now().UTC()).
			Count(&futureApptCount).Error; err != nil {
			return err
		}
		if futureApptCount > 0 {
			return httperr.ErrBusiness("future_appointments_exist")
		}

		// 5. Nular notes de agendamentos passados (podem conter PII textual)
		if err := tx.Model(&models.Appointment{}).
			Where("barbershop_id = ? AND client_id = ?", barbershopID, clientID).
			Update("notes", nil).Error; err != nil {
			return err
		}

		// 6. Deletar métricas comportamentais (sem valor contábil)
		if err := tx.Where("barbershop_id = ? AND client_id = ?", barbershopID, clientID).
			Delete(&models.ClientMetrics{}).Error; err != nil {
			return err
		}

		// 7. Deletar categoria CRM (dado classificatório sem valor após anonimização)
		if err := tx.Exec(
			"DELETE FROM client_crm_categories WHERE barbershop_id = ? AND client_id = ?",
			barbershopID, clientID,
		).Error; err != nil {
			return err
		}

		// 8. Anonimizar o cliente — sobrescrever PII, manter ID para integridade referencial
		now := time.Now().UTC()
		if err := tx.Model(&client).Updates(map[string]any{
			"name":              "Cliente removido",
			"phone":             nil,
			"email":             nil,
			"anonymized_at":     now,
			"anonymized_reason": reason,
		}).Error; err != nil {
			return err
		}

		return nil
	})
}

// DispatchAudit deve ser chamado APÓS a transação confirmar com sucesso.
// Separado do Execute para garantir que o evento de auditoria só seja
// disparado quando a transação realmente commitou.
func (uc *AnonymizeClient) DispatchAudit(barbershopID, clientID, userID uint) {
	if uc.audit == nil {
		return
	}
	uid := userID
	cid := clientID
	uc.audit.Dispatch(audit.Event{
		BarbershopID: barbershopID,
		UserID:       &uid,
		Action:       "client_anonymized",
		Entity:       "client",
		EntityID:     &cid,
		// Sem dados pessoais no metadata (LGPD)
	})
}
