package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/notification"
	"github.com/BruksfildServices01/barber-scheduler/internal/timezone"
)

// ── Evolution API Client ──────────────────────────────────────────────────────

type EvolutionClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewEvolutionClient(baseURL, apiKey string) *EvolutionClient {
	return &EvolutionClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *EvolutionClient) do(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var buf *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		buf = bytes.NewReader(b)
	} else {
		buf = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("apikey", c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.httpClient.Do(req)
}

// CreateInstance cria uma instância na Evolution API.
func (c *EvolutionClient) CreateInstance(ctx context.Context, instanceName string) error {
	resp, err := c.do(ctx, http.MethodPost, "/instance/create", map[string]any{
		"instanceName": instanceName,
		"qrcode":       true,
	})
	if err != nil {
		return fmt.Errorf("evolution: create instance: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("evolution: create instance status %d", resp.StatusCode)
	}
	return nil
}

// QRCodeResponse é a resposta do endpoint de conexão.
type QRCodeResponse struct {
	Base64 string `json:"base64"` // data:image/png;base64,...
	Code   string `json:"code"`   // string raw do QR
}

// GetQRCode busca o QR code para escanear com o WhatsApp.
func (c *EvolutionClient) GetQRCode(ctx context.Context, instanceName string) (*QRCodeResponse, error) {
	resp, err := c.do(ctx, http.MethodGet, "/instance/connect/"+instanceName, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		QRCode QRCodeResponse `json:"qrcode"`
		// v1.7.4 retorna direto no root em alguns casos
		Base64 string `json:"base64"`
		Code   string `json:"code"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	// Normaliza resposta de diferentes versões da API
	qr := &result.QRCode
	if qr.Base64 == "" {
		qr.Base64 = result.Base64
	}
	if qr.Code == "" {
		qr.Code = result.Code
	}
	return qr, nil
}

// ConnectionState retorna "open" quando conectado, "close" quando não.
func (c *EvolutionClient) ConnectionState(ctx context.Context, instanceName string) (string, error) {
	resp, err := c.do(ctx, http.MethodGet, "/instance/connectionState/"+instanceName, nil)
	if err != nil {
		return "error", err
	}
	defer resp.Body.Close()

	var result struct {
		Instance struct {
			State string `json:"state"`
		} `json:"instance"`
		State string `json:"state"` // fallback v1
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "error", err
	}
	state := result.Instance.State
	if state == "" {
		state = result.State
	}
	return state, nil
}

// SetWebhook configura o webhook da instância.
func (c *EvolutionClient) SetWebhook(ctx context.Context, instanceName, webhookURL string) error {
	resp, err := c.do(ctx, http.MethodPost, "/webhook/set/"+instanceName, map[string]any{
		"url":               webhookURL,
		"webhook_by_events": false,
		"events":            []string{"MESSAGES_UPSERT"},
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// GetConnectedPhone retorna o número de telefone conectado na instância.
// Retorna vazio se a instância não estiver conectada ou o número não estiver disponível.
func (c *EvolutionClient) GetConnectedPhone(ctx context.Context, instanceName string) (string, error) {
	resp, err := c.do(ctx, http.MethodGet, "/instance/fetchInstances", nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var instances []struct {
		InstanceName string `json:"instanceName"`
		OwnerJid     string `json:"ownerJid"`
		ProfileName  string `json:"profileName"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&instances); err != nil {
		return "", err
	}

	for _, inst := range instances {
		if inst.InstanceName == instanceName && inst.OwnerJid != "" {
			// ownerJid format: "5511999999999@s.whatsapp.net"
			return extractPhone(inst.OwnerJid), nil
		}
	}
	return "", nil
}

func extractPhone(jid string) string {
	parts := strings.Split(jid, "@")
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

// GetPairingCode retorna o código de 8 dígitos para conectar pelo telefone.
// O usuário digita esse código no WhatsApp em vez de escanear o QR code.
func (c *EvolutionClient) GetPairingCode(ctx context.Context, instanceName, phoneNumber string) (string, error) {
	phone := sanitizePhone(phoneNumber)
	resp, err := c.do(ctx, http.MethodPost, "/instance/pairing-code/"+instanceName, map[string]any{
		"phoneNumber": phone,
	})
	if err != nil {
		return "", fmt.Errorf("evolution: pairing code: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code       string `json:"code"`
		PairingCode string `json:"pairingCode"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	code := result.Code
	if code == "" {
		code = result.PairingCode
	}
	if code == "" {
		return "", fmt.Errorf("evolution: pairing code empty in response")
	}
	return code, nil
}

// DeleteInstance remove a instância da Evolution API.
func (c *EvolutionClient) DeleteInstance(ctx context.Context, instanceName string) error {
	resp, err := c.do(ctx, http.MethodDelete, "/instance/delete/"+instanceName, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// SendMedia envia uma imagem com legenda para um número WhatsApp.
func (c *EvolutionClient) SendMedia(ctx context.Context, instanceName, number, imageURL, caption string) error {
	resp, err := c.do(ctx, http.MethodPost, "/message/sendMedia/"+instanceName, map[string]any{
		"number": sanitizePhone(number),
		"mediaMessage": map[string]any{
			"mediatype": "image",
			"caption":   caption,
			"media":     imageURL,
		},
	})
	if err != nil {
		return fmt.Errorf("evolution: send media: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("evolution: send media status %d", resp.StatusCode)
	}
	return nil
}

// SendText envia mensagem de texto para um número WhatsApp.
func (c *EvolutionClient) SendText(ctx context.Context, instanceName, number, text string) error {
	resp, err := c.do(ctx, http.MethodPost, "/message/sendText/"+instanceName, map[string]any{
		"number": sanitizePhone(number),
		"text":   text,
	})
	if err != nil {
		return fmt.Errorf("evolution: send text: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("evolution: send text status %d", resp.StatusCode)
	}
	return nil
}

// sanitizePhone normaliza qualquer formato de telefone brasileiro para DDI+DDD+número.
// Exemplos: "(11) 91354-0401" → "5511913540401"
//           "+55 11 91354-0401" → "5511913540401"
//           "011913540401" → "5511913540401"
func sanitizePhone(phone string) string {
	// Remove tudo exceto dígitos
	digits := ""
	for _, ch := range phone {
		if ch >= '0' && ch <= '9' {
			digits += string(ch)
		}
	}

	// Remove zeros à esquerda (formato antigo 011...)
	for strings.HasPrefix(digits, "0") {
		digits = digits[1:]
	}

	// Já tem código do país
	if strings.HasPrefix(digits, "55") && len(digits) >= 12 {
		return digits
	}

	// Número brasileiro sem prefixo (10 ou 11 dígitos)
	return "55" + digits
}

// ── WhatsAppNotifier ──────────────────────────────────────────────────────────

type WhatsAppNotifier struct {
	evolutionURL string
	evolutionKey string
	appURL       string
}

func NewWhatsAppNotifier(evolutionURL, evolutionKey, appURL string) *WhatsAppNotifier {
	return &WhatsAppNotifier{
		evolutionURL: evolutionURL,
		evolutionKey: evolutionKey,
		appURL:       appURL,
	}
}

func (n *WhatsAppNotifier) clientFor(instanceName string) *EvolutionClient {
	return NewEvolutionClient(n.evolutionURL, n.evolutionKey)
}

// instanceNameForBarbershop retorna o nome da instância — centralizado aqui
// para manter coerência com o handler.
func instanceNameForBarbershop(barbershopID uint) string {
	return fmt.Sprintf("bs%d", barbershopID)
}

func (n *WhatsAppNotifier) send(ctx context.Context, barbershopID uint, phone, msg string) {
	if phone == "" || n.evolutionURL == "" {
		return
	}
	instance := instanceNameForBarbershop(barbershopID)
	client := n.clientFor(instance)
	if err := client.SendText(ctx, instance, phone, msg); err != nil {
		log.Printf("[WhatsApp] send failed barbershop=%d phone=%s: %v", barbershopID, phone, err)
	}
}

func (n *WhatsAppNotifier) NotifyConfirmed(ctx context.Context, in domain.AppointmentConfirmedInput) error {
	if in.ClientPhone == "" {
		return nil
	}
	loc := timezone.Location(in.Timezone)
	start := in.StartTime.In(loc)
	end := in.EndTime.In(loc)

	lines := []string{
		fmt.Sprintf("✅ *Agendamento confirmado, %s!*", in.ClientName),
		"",
		fmt.Sprintf("✂️ *%s*", in.ServiceName),
		fmt.Sprintf("📅 %s", formatDate(start)),
		fmt.Sprintf("🕐 %s – %s", formatTime(start), formatTime(end)),
	}
	if in.TicketURL != "" {
		lines = append(lines, "", "🔗 *Seu ticket (cancelar ou remarcar):*", in.TicketURL)
	}
	lines = append(lines,
		"", "📌 *Lembre-se:*",
		"• Chegue 5 minutos antes",
		"• Faltas sem aviso afetam prioridade futura",
		"• Cancelamentos pelo link acima",
	)
	if in.BarbershopPhone != "" {
		lines = append(lines, "", fmt.Sprintf("📞 Dúvidas: %s", in.BarbershopPhone))
	}
	lines = append(lines, "", fmt.Sprintf("_Mensagem automática · %s_", in.BarbershopName))

	// TODO: buscar barbershop_id pelo slug quando necessário
	// Por ora usa o notifier apenas quando barbershop_id está disponível via contexto
	_ = strings.Join(lines, "\n")
	return nil
}

func (n *WhatsAppNotifier) NotifyCancelled(ctx context.Context, in domain.AppointmentCancelledInput) error {
	return nil
}

func (n *WhatsAppNotifier) NotifyRescheduled(ctx context.Context, in domain.AppointmentRescheduledInput) error {
	return nil
}

// ── Formatters ────────────────────────────────────────────────────────────────

var weekdaysPT = [...]string{"domingo", "segunda-feira", "terça-feira", "quarta-feira", "quinta-feira", "sexta-feira", "sábado"}
var monthsPT   = [...]string{"janeiro", "fevereiro", "março", "abril", "maio", "junho", "julho", "agosto", "setembro", "outubro", "novembro", "dezembro"}

func formatDate(t time.Time) string {
	return fmt.Sprintf("%s, %d de %s", weekdaysPT[t.Weekday()], t.Day(), monthsPT[t.Month()-1])
}

func formatTime(t time.Time) string {
	return fmt.Sprintf("%02d:%02d", t.Hour(), t.Minute())
}
