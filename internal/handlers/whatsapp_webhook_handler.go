package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/infra/notification"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
	"github.com/BruksfildServices01/barber-scheduler/internal/timezone"
)

// WhatsAppWebhookHandler recebe mensagens da Evolution API e responde
// automaticamente com o comprovante de agendamento do cliente.
type WhatsAppWebhookHandler struct {
	db           *gorm.DB
	evolutionURL string
	evolutionKey string
}

func NewWhatsAppWebhookHandler(db *gorm.DB, evolutionURL, evolutionKey string) *WhatsAppWebhookHandler {
	return &WhatsAppWebhookHandler{db: db, evolutionURL: evolutionURL, evolutionKey: evolutionKey}
}

type evolutionWebhookPayload struct {
	Event    string `json:"event"`
	Instance string `json:"instance"`
	Data     struct {
		Key struct {
			RemoteJid string `json:"remoteJid"`
			FromMe    bool   `json:"fromMe"`
		} `json:"key"`
		Message struct {
			Conversation        string `json:"conversation"`
			ExtendedTextMessage struct {
				Text string `json:"text"`
			} `json:"extendedTextMessage"`
		} `json:"message"`
	} `json:"data"`
}

// Receive recebe o webhook da Evolution API.
// POST /api/webhooks/whatsapp
func (h *WhatsAppWebhookHandler) Receive(c *gin.Context) {
	var payload evolutionWebhookPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	// Ignora mensagens enviadas por nós, grupos e eventos que não são mensagens
	if payload.Data.Key.FromMe ||
		payload.Event != "messages.upsert" ||
		strings.HasSuffix(payload.Data.Key.RemoteJid, "@g.us") {
		c.Status(http.StatusOK)
		return
	}

	clientPhone := extractPhone(payload.Data.Key.RemoteJid)
	if clientPhone == "" {
		c.Status(http.StatusOK)
		return
	}

	// Busca instância pelo nome — identifica qual barbearia recebeu a mensagem
	var inst models.BarbershopWhatsAppInstance
	if err := h.db.WithContext(c.Request.Context()).
		Where("instance_name = ?", payload.Instance).
		First(&inst).Error; err != nil {
		c.Status(http.StatusOK)
		return
	}

	go h.processMessage(inst, clientPhone)
	c.Status(http.StatusOK)
}

// ── Processamento assíncrono ───────────────────────────────────────────────────

type appointmentForReply struct {
	ClientName      string    `gorm:"column:client_name"`
	ClientPhone     string    `gorm:"column:client_phone"`
	BarbershopName  string    `gorm:"column:barbershop_name"`
	BarbershopPhone string    `gorm:"column:barbershop_phone"`
	ServiceName     string    `gorm:"column:service_name"`
	StartTime       time.Time `gorm:"column:start_time"`
	EndTime         time.Time `gorm:"column:end_time"`
	Timezone        string    `gorm:"column:timezone"`
	Token           string    `gorm:"column:token"`
}

func (h *WhatsAppWebhookHandler) processMessage(inst models.BarbershopWhatsAppInstance, clientPhone string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Últimos 8 dígitos do telefone para busca tolerante a formatação
	suffix := clientPhone
	if len(suffix) > 8 {
		suffix = suffix[len(suffix)-8:]
	}

	var data appointmentForReply
	err := h.db.WithContext(ctx).Raw(`
		SELECT
			c.name  AS client_name,
			c.phone AS client_phone,
			b.name  AS barbershop_name,
			b.phone AS barbershop_phone,
			bs.name AS service_name,
			a.start_time,
			a.end_time,
			b.timezone,
			COALESCE(t.token, '') AS token
		FROM clients c
		JOIN appointments a
			ON  a.client_id    = c.id
			AND a.barbershop_id = ?
			AND a.status IN ('scheduled', 'awaiting_payment')
			AND a.start_time >= NOW()
		JOIN barbershops b
			ON b.id = a.barbershop_id
		JOIN barbershop_services bs
			ON bs.id = a.barber_product_id
		LEFT JOIN appointment_tickets t
			ON t.appointment_id = a.id AND t.expires_at > NOW()
		WHERE c.barbershop_id = ?
		  AND REGEXP_REPLACE(c.phone, '[^0-9]', '', 'g') LIKE ?
		ORDER BY a.start_time ASC
		LIMIT 1
	`, inst.BarbershopID, inst.BarbershopID, "%"+suffix).Scan(&data).Error

	if err != nil || data.ClientName == "" {
		log.Printf("[WhatsApp webhook] no active appointment for phone %s (barbershop %d)", clientPhone, inst.BarbershopID)
		return
	}

	client := notification.NewEvolutionClient(h.evolutionURL, h.evolutionKey)
	msg := h.buildReply(data)
	if err := client.SendText(ctx, inst.InstanceName, clientPhone, msg); err != nil {
		log.Printf("[WhatsApp webhook] reply failed for %s: %v", clientPhone, err)
	}
}

func (h *WhatsAppWebhookHandler) buildReply(d appointmentForReply) string {
	loc := timezone.Location(d.Timezone)
	start := d.StartTime.In(loc)
	end := d.EndTime.In(loc)

	weekdays := [...]string{"domingo", "segunda-feira", "terça-feira", "quarta-feira", "quinta-feira", "sexta-feira", "sábado"}
	months   := [...]string{"janeiro", "fevereiro", "março", "abril", "maio", "junho", "julho", "agosto", "setembro", "outubro", "novembro", "dezembro"}

	dateStr := fmt.Sprintf("%s, %d de %s", weekdays[start.Weekday()], start.Day(), months[start.Month()-1])
	timeStr := fmt.Sprintf("%02d:%02d – %02d:%02d", start.Hour(), start.Minute(), end.Hour(), end.Minute())

	lines := []string{
		fmt.Sprintf("✅ *Olá, %s! Comprovante recebido.*", d.ClientName),
		"",
		"Confirmamos seu agendamento:",
		"",
		fmt.Sprintf("✂️ *%s*", d.ServiceName),
		fmt.Sprintf("📅 %s", dateStr),
		fmt.Sprintf("🕐 %s", timeStr),
	}

	if d.Token != "" {
		lines = append(lines,
			"",
			"🔗 *Seu ticket (cancelar ou remarcar):*",
			fmt.Sprintf("https://corteon.app/ticket/%s", d.Token),
		)
	}

	lines = append(lines,
		"",
		"📌 *Informações importantes:*",
		"• Chegue 5 min antes do horário",
		"• Faltas sem aviso afetam sua prioridade futura",
		"• Cancelamentos pelo link acima",
	)

	if d.BarbershopPhone != "" {
		lines = append(lines, "", fmt.Sprintf("📞 Dúvidas: %s", d.BarbershopPhone))
	}

	lines = append(lines, "", fmt.Sprintf("_Mensagem automática · %s_", d.BarbershopName))
	return strings.Join(lines, "\n")
}

func extractPhone(remoteJid string) string {
	parts := strings.Split(remoteJid, "@")
	if len(parts) == 0 {
		return ""
	}
	digits := ""
	for _, ch := range parts[0] {
		if ch >= '0' && ch <= '9' {
			digits += string(ch)
		}
	}
	return digits
}
