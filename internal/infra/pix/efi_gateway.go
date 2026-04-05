package pix

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/payment"
)

// EfiGateway integra com a API PIX da Efí (ex-Gerencianet).
// Documentação: https://dev.efipay.com.br/docs/api-pix/
//
// Para usar em produção:
//  1. Gere credenciais em https://sejaefi.com.br → API → Aplicações
//  2. Configure EFI_CLIENT_ID, EFI_CLIENT_SECRET e EFI_PIX_KEY no ambiente
//  3. Defina PIX_PROVIDER=efi no ambiente
type EfiGateway struct {
	clientID     string
	clientSecret string
	pixKey       string
	baseURL      string
	httpClient   *http.Client
}

const (
	efiProductionURL = "https://pix.api.efipay.com.br"
	efiSandboxURL    = "https://pix-h.api.efipay.com.br"
)

// NewEfiGateway cria o gateway Efí.
// sandbox=true usa o ambiente de homologação.
func NewEfiGateway(clientID, clientSecret, pixKey string, sandbox bool) *EfiGateway {
	baseURL := efiProductionURL
	if sandbox {
		baseURL = efiSandboxURL
	}
	return &EfiGateway{
		clientID:     clientID,
		clientSecret: clientSecret,
		pixKey:       pixKey,
		baseURL:      baseURL,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
	}
}

// getToken obtém um access token OAuth2 via client_credentials.
func (g *EfiGateway) getToken() (string, error) {
	body := bytes.NewBufferString("grant_type=client_credentials")
	req, err := http.NewRequest(http.MethodPost, g.baseURL+"/oauth/token", body)
	if err != nil {
		return "", err
	}
	creds := base64.StdEncoding.EncodeToString([]byte(g.clientID + ":" + g.clientSecret))
	req.Header.Set("Authorization", "Basic "+creds)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("efi oauth: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("efi oauth status %d: %s", resp.StatusCode, raw)
	}

	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.AccessToken, nil
}

// CreateCharge cria uma cobrança PIX imediata (cob) na API Efí.
func (g *EfiGateway) CreateCharge(amount float64, description string) (*domain.PixCharge, error) {
	token, err := g.getToken()
	if err != nil {
		return nil, fmt.Errorf("efi get token: %w", err)
	}

	// Monta payload da cobrança PIX imediata
	payload := map[string]any{
		"calendario": map[string]any{
			"expiracao": 900, // 15 minutos
		},
		"valor": map[string]any{
			"original": fmt.Sprintf("%.2f", amount/100), // converte centavos → reais
		},
		"chave":    g.pixKey,
		"infoAdicionais": []map[string]string{
			{"nome": "Serviço", "valor": description},
		},
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, g.baseURL+"/v2/cob", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("efi create cob: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("efi create cob status %d: %s", resp.StatusCode, raw)
	}

	var out struct {
		TxID     string `json:"txid"`
		PixCopiaECola string `json:"pixCopiaECola"`
		Calendario struct {
			Expiracao int `json:"expiracao"`
		} `json:"calendario"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	expiresAt := time.Now().UTC().Add(time.Duration(out.Calendario.Expiracao) * time.Second)
	if out.Calendario.Expiracao == 0 {
		expiresAt = time.Now().UTC().Add(15 * time.Minute)
	}

	return &domain.PixCharge{
		TxID:      out.TxID,
		QRCode:    out.PixCopiaECola,
		ExpiresAt: expiresAt,
	}, nil
}
