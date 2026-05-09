// Package calendar integra com o Google Calendar API via HTTP direto (sem SDK oficial).
// Documentação: https://developers.google.com/calendar/api/v3/reference
package calendar

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	googleAuthURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL = "https://oauth2.googleapis.com/token"
	calendarAPIURL = "https://www.googleapis.com/calendar/v3/calendars/primary/events"

	// Escopo mínimo: criar/editar eventos na agenda principal do usuário.
	calendarScope = "https://www.googleapis.com/auth/calendar.events"
)

var httpClient = &http.Client{Timeout: 15 * time.Second}

// OAuthConfig guarda as credenciais do app Google.
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// TokenResult é retornado pelo endpoint de troca de código.
type TokenResult struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresIn    int       `json:"expires_in"` // segundos
	Expiry       time.Time // calculado localmente
}

// AuthURL monta a URL de autorização OAuth para o Google.
func AuthURL(cfg OAuthConfig, state string) string {
	params := url.Values{}
	params.Set("client_id", cfg.ClientID)
	params.Set("redirect_uri", cfg.RedirectURL)
	params.Set("response_type", "code")
	params.Set("scope", calendarScope)
	params.Set("access_type", "offline")  // necessário para obter refresh_token
	params.Set("prompt", "consent")       // força consentimento para garantir refresh_token
	params.Set("state", state)
	return googleAuthURL + "?" + params.Encode()
}

// ExchangeCode troca o código de autorização por access_token + refresh_token.
func ExchangeCode(ctx context.Context, cfg OAuthConfig, code string) (*TokenResult, error) {
	body := url.Values{}
	body.Set("grant_type", "authorization_code")
	body.Set("client_id", cfg.ClientID)
	body.Set("client_secret", cfg.ClientSecret)
	body.Set("redirect_uri", cfg.RedirectURL)
	body.Set("code", code)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, googleTokenURL,
		strings.NewReader(body.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google token exchange: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("google token exchange %d: %s", resp.StatusCode, string(data))
	}

	var result TokenResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("google token parse: %w", err)
	}
	result.Expiry = time.Now().UTC().Add(time.Duration(result.ExpiresIn) * time.Second)
	return &result, nil
}

// RefreshAccessToken usa o refresh_token para obter um novo access_token.
func RefreshAccessToken(ctx context.Context, cfg OAuthConfig, refreshToken string) (*TokenResult, error) {
	body := url.Values{}
	body.Set("grant_type", "refresh_token")
	body.Set("client_id", cfg.ClientID)
	body.Set("client_secret", cfg.ClientSecret)
	body.Set("refresh_token", refreshToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, googleTokenURL,
		strings.NewReader(body.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google refresh: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("google refresh %d: %s", resp.StatusCode, string(data))
	}

	var result TokenResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("google refresh parse: %w", err)
	}
	result.Expiry = time.Now().UTC().Add(time.Duration(result.ExpiresIn) * time.Second)
	// refresh_token não é retornado no refresh — mantém o original
	result.RefreshToken = refreshToken
	return &result, nil
}

// EventInput são os dados necessários para criar um evento no Google Calendar.
type EventInput struct {
	Summary     string    // "✂️ Corte — João Silva"
	Description string    // detalhes opcionais
	Start       time.Time
	End         time.Time
	Timezone    string // ex: "America/Sao_Paulo"
}

type calendarDateTime struct {
	DateTime string `json:"dateTime"`
	TimeZone string `json:"timeZone"`
}

type calendarReminder struct {
	Method  string `json:"method"`
	Minutes int    `json:"minutes"`
}

type calendarReminders struct {
	UseDefault bool               `json:"useDefault"`
	Overrides  []calendarReminder `json:"overrides"`
}

type calendarEventRequest struct {
	Summary     string            `json:"summary"`
	Description string            `json:"description,omitempty"`
	Start       calendarDateTime  `json:"start"`
	End         calendarDateTime  `json:"end"`
	Reminders   calendarReminders `json:"reminders"`
}

// CreateEvent cria um evento na agenda principal do barbeiro.
// accessToken deve ser um token válido (não expirado).
func CreateEvent(ctx context.Context, accessToken string, input EventInput) error {
	tz := input.Timezone
	if tz == "" {
		tz = "America/Sao_Paulo"
	}

	event := calendarEventRequest{
		Summary:     input.Summary,
		Description: input.Description,
		Start: calendarDateTime{
			DateTime: input.Start.Format(time.RFC3339),
			TimeZone: tz,
		},
		End: calendarDateTime{
			DateTime: input.End.Format(time.RFC3339),
			TimeZone: tz,
		},
		Reminders: calendarReminders{
			UseDefault: false,
			Overrides: []calendarReminder{
				{Method: "popup", Minutes: 60},
				{Method: "popup", Minutes: 15},
			},
		},
	}

	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("google calendar marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, calendarAPIURL,
		bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("google calendar create: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("google calendar %d: %s", resp.StatusCode, string(data))
	}
	return nil
}
