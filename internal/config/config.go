package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	// =========================
	// CORE
	// =========================
	DBUrl      string
	JWTSecret  string
	ServerPort string
	AppURL     string // frontend base URL, e.g. https://app.seudominio.com

	// =========================
	// CORS
	// =========================
	// Allowlist de origens (CSV): "http://localhost:4200,https://app.seudominio.com"
	// Se vazio, não libera CORS (recomendado falhar fechado).
	CORSAllowedOrigins []string

	// =========================
	// EMAIL (BREVO)
	// =========================
	EmailEnabled  bool
	EmailFrom     string
	BrevoAPIKey   string

	// SMTP (legado — ignorado quando BrevoAPIKey está definido)
	SMTPHost string
	SMTPPort string
	SMTPUser string
	SMTPPass string

	// =========================
	// REDIS (rate limiter distribuído)
	// =========================
	// RedisURL: "redis://localhost:6379" ou vazio → usa rate limiter in-memory.
	RedisURL string

	// =========================
	// MERCADO PAGO
	// =========================
	// MPProvider: "mock" (padrão) | "mp"
	MPProvider    string
	MPAccessToken string
	// BackendURL é usado para montar a notification_url enviada ao Mercado Pago.
	// Ex: https://api.seudominio.com
	BackendURL string

	// =========================
	// SAAS BILLING
	// =========================
	// Preço mensal da plataforma em centavos (padrão: 2190 = R$21,90).
	PlatformMonthlyPriceCents int
	// Public key do MP da plataforma (exibida no frontend para Checkout Transparente).
	PlatformMPPublicKey string
	// Duração do período de trial em dias (padrão: 7).
	TrialDays int

	// =========================
	// CLOUDFLARE R2 (storage)
	// =========================
	R2AccountID       string
	R2AccessKeyID     string
	R2SecretAccessKey string
	R2BucketName      string
	R2PublicURL       string // ex: https://pub-xxx.r2.dev
}

func Load() *Config {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("❌ DATABASE_URL não definida")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("❌ JWT_SECRET não definida")
	}

	cfg := &Config{
		// CORE
		DBUrl:      dbURL,
		JWTSecret:  jwtSecret,
		ServerPort: getEnv("SERVER_PORT", "8080"),
		AppURL:     getEnv("APP_URL", "http://localhost:3000"),

		// CORS
		CORSAllowedOrigins: splitCSV(getEnv("CORS_ALLOWED_ORIGINS", "")),

		// EMAIL
		EmailEnabled: getEnv("EMAIL_ENABLED", "false") == "true",
		EmailFrom:    getEnv("EMAIL_FROM", ""),
		BrevoAPIKey:  getEnv("BREVO_API_KEY", ""),

		SMTPHost: getEnv("SMTP_HOST", ""),
		SMTPPort: getEnv("SMTP_PORT", ""),
		SMTPUser: getEnv("SMTP_USER", ""),
		SMTPPass: getEnv("SMTP_PASS", ""),

		// REDIS
		RedisURL: getEnv("REDIS_URL", ""),

		// MERCADO PAGO
		MPProvider:    getEnv("MP_PROVIDER", "mock"),
		MPAccessToken: getEnv("MP_ACCESS_TOKEN", ""),
		BackendURL:    strings.TrimRight(getEnv("BACKEND_URL", "http://localhost:8080"), "/"),

		// SAAS BILLING
		PlatformMonthlyPriceCents: getEnvInt("PLATFORM_MONTHLY_PRICE_CENTS", 2190),
		PlatformMPPublicKey:       getEnv("PLATFORM_MP_PUBLIC_KEY", ""),
		TrialDays:                 getEnvInt("TRIAL_DAYS", 30),

		// R2
		R2AccountID:       getEnv("R2_ACCOUNT_ID", ""),
		R2AccessKeyID:     getEnv("R2_ACCESS_KEY_ID", ""),
		R2SecretAccessKey: getEnv("R2_SECRET_ACCESS_KEY", ""),
		R2BucketName:      getEnv("R2_BUCKET_NAME", ""),
		R2PublicURL:       strings.TrimRight(getEnv("R2_PUBLIC_URL", ""), "/"),
	}

	cfg.AppURL = strings.TrimRight(cfg.AppURL, "/")

	// =========================
	// VALIDAÇÃO DE EMAIL
	// =========================
	if cfg.EmailEnabled {
		if cfg.EmailFrom == "" {
			log.Fatal("❌ EMAIL_ENABLED=true mas EMAIL_FROM não definido")
		}
		if cfg.BrevoAPIKey == "" && (cfg.SMTPHost == "" || cfg.SMTPPort == "" || cfg.SMTPUser == "" || cfg.SMTPPass == "") {
			log.Fatal("❌ EMAIL_ENABLED=true mas nem BREVO_API_KEY nem variáveis SMTP estão completas")
		}
	}

	log.Println("[CONFIG] EMAIL_ENABLED =", cfg.EmailEnabled)

	if len(cfg.CORSAllowedOrigins) > 0 {
		log.Println("[CONFIG] CORS_ALLOWED_ORIGINS =", strings.Join(cfg.CORSAllowedOrigins, ","))
	} else {
		log.Println("[CONFIG] CORS_ALLOWED_ORIGINS = (empty)")
	}

	return cfg
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func splitCSV(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return []string{}
	}

	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (c *Config) Addr() string {
	return fmt.Sprintf(":%s", c.ServerPort)
}
