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

// maskPhone mascara um número de telefone para logs, mantendo apenas os
// últimos 4 dígitos visíveis. Nunca loga número completo (LGPD).
// Aceita número puro ou JID do WhatsApp ("5511999991234@s.whatsapp.net").
// Exemplos: "5511999991234" → "**********1234"
func maskPhone(raw string) string {
	// Extrai só dígitos (aceita também JID como "5511...@s.whatsapp.net")
	digits := ""
	for _, ch := range strings.Split(raw, "@")[0] {
		if ch >= '0' && ch <= '9' {
			digits += string(ch)
		}
	}
	if len(digits) <= 4 {
		return strings.Repeat("*", len(digits))
	}
	return strings.Repeat("*", len(digits)-4) + digits[len(digits)-4:]
}

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
	// Valida autenticidade do webhook verificando o header "apikey" enviado
	// pela Evolution API — mesmo valor configurado em EVOLUTION_API_KEY.
	// Só valida se a chave estiver configurada (evita bloquear em dev sem Evolution).
	if h.evolutionKey != "" {
		if c.GetHeader("apikey") != h.evolutionKey {
			// Não loga o valor recebido para não vazar chaves em logs
			log.Printf("[WhatsApp webhook] header apikey inválido — requisição rejeitada")
			c.Status(http.StatusUnauthorized)
			return
		}
	}

	var payload evolutionWebhookPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	log.Printf("[WhatsApp webhook] event=%q instance=%q", payload.Event, payload.Instance)

	// Ignora mensagens enviadas por nós, grupos e eventos que não são mensagens
	// v1.x usa "MESSAGES_UPSERT", v2.x usa "messages.upsert"
	eventLower := strings.ToLower(payload.Event)
	if payload.Data.Key.FromMe ||
		(eventLower != "messages.upsert" && eventLower != "messages_upsert") ||
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
	ClientName       string    `gorm:"column:client_name"`
	ClientPhone      string    `gorm:"column:client_phone"`
	BarbershopName   string    `gorm:"column:barbershop_name"`
	BarbershopPhone  string    `gorm:"column:barbershop_phone"`
	ServiceName      string    `gorm:"column:service_name"`
	ServiceImageURL  string    `gorm:"column:service_image_url"`
	StartTime        time.Time `gorm:"column:start_time"`
	EndTime          time.Time `gorm:"column:end_time"`
	Timezone         string    `gorm:"column:timezone"`
	Token            string    `gorm:"column:token"`
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
			COALESCE(
				(SELECT url FROM barbershop_service_images
				 WHERE service_id = bs.id
				 ORDER BY position ASC LIMIT 1),
				''
			)       AS service_image_url,
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
		log.Printf("[WhatsApp webhook] no active appointment for phone %s (barbershop %d)", maskPhone(clientPhone), inst.BarbershopID)
		return
	}

	client := notification.NewEvolutionClient(h.evolutionURL, h.evolutionKey)
	msg := h.buildReply(data)

	var sendErr error
	if data.ServiceImageURL != "" {
		// Envia foto do serviço com a mensagem como legenda
		sendErr = client.SendMedia(ctx, inst.InstanceName, clientPhone, data.ServiceImageURL, msg)
	}
	// Fallback para texto puro se não tiver imagem ou se o envio de mídia falhar
	if data.ServiceImageURL == "" || sendErr != nil {
		if sendErr != nil {
			log.Printf("[WhatsApp webhook] media send failed, falling back to text: %v", sendErr)
		}
		sendErr = client.SendText(ctx, inst.InstanceName, clientPhone, msg)
	}
	if sendErr != nil {
		log.Printf("[WhatsApp webhook] reply failed for %s: %v", maskPhone(clientPhone), sendErr)
	}
}

func (h *WhatsAppWebhookHandler) buildReply(d appointmentForReply) string {
	loc := timezone.Location(d.Timezone)
	start := d.StartTime.In(loc)
	end := d.EndTime.In(loc)

	weekdays := [...]string{"Domingo", "Segunda-feira", "Terça-feira", "Quarta-feira", "Quinta-feira", "Sexta-feira", "Sábado"}
	months   := [...]string{"janeiro", "fevereiro", "março", "abril", "maio", "junho", "julho", "agosto", "setembro", "outubro", "novembro", "dezembro"}

	dateStr := fmt.Sprintf("%s, %d de %s", weekdays[start.Weekday()], start.Day(), months[start.Month()-1])
	timeStr := fmt.Sprintf("%02d:%02d – %02d:%02d", start.Hour(), start.Minute(), end.Hour(), end.Minute())

	lines := []string{
		fmt.Sprintf("✅ *Agendamento confirmado, %s!*", d.ClientName),
		"",
		fmt.Sprintf("✂️  *%s*", d.ServiceName),
		fmt.Sprintf("📅  %s", dateStr),
		fmt.Sprintf("🕐  %s", timeStr),
		"",
		"━━━━━━━━━━━━━━━━━",
	}

	if d.Token != "" {
		lines = append(lines,
			"",
			"🎫 *Seu ticket:*",
			fmt.Sprintf("https://corteon.app/ticket/%s", d.Token),
			"_(cancelar ou remarcar pelo link acima)_",
		)
	}

	lines = append(lines,
		"",
		"━━━━━━━━━━━━━━━━━",
		"",
		"📌 *Lembre-se:*",
		"• Chegue 5 minutos antes",
		"• Faltas sem aviso afetam sua prioridade",
	)

	if d.BarbershopPhone != "" {
		lines = append(lines, "", fmt.Sprintf("📞  *Dúvidas:* %s", d.BarbershopPhone))
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

	// WhatsApp às vezes inclui o código do servidor na frente (ex: "21664" antes do DDI)
	// Normaliza para número brasileiro: remove prefixo até encontrar "55" + DDD (2 dígitos) + número (8-9 dígitos)
	if len(digits) > 13 {
		// Tenta encontrar "55" seguido de DDD válido
		for i := 0; i <= len(digits)-13; i++ {
			if digits[i] == '5' && i+1 < len(digits) && digits[i+1] == '5' {
				candidate := digits[i:]
				if len(candidate) >= 12 && len(candidate) <= 13 {
					return candidate
				}
				if len(candidate) > 13 {
					return candidate[:13]
				}
			}
		}
	}
	return digits
}
