package config

import (
	"fmt"
	"log"
	"os"
	"strings"
)

type Config struct {
	// =========================
	// CORE
	// =========================
	DBUrl      string
	JWTSecret  string
	ServerPort string

	// =========================
	// CORS
	// =========================
	// Allowlist de origens (CSV): "http://localhost:4200,https://app.seudominio.com"
	// Se vazio, não libera CORS (recomendado falhar fechado).
	CORSAllowedOrigins []string

	// =========================
	// EMAIL (BREVO SMTP)
	// =========================
	EmailEnabled bool
	EmailFrom    string

	SMTPHost string
	SMTPPort string
	SMTPUser string
	SMTPPass string

	// =========================
	// PIX
	// =========================
	PixWebhookSecret string
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

		// CORS
		CORSAllowedOrigins: splitCSV(getEnv("CORS_ALLOWED_ORIGINS", "")),

		// EMAIL
		EmailEnabled: getEnv("EMAIL_ENABLED", "false") == "true",
		EmailFrom:    getEnv("EMAIL_FROM", ""),

		SMTPHost: getEnv("SMTP_HOST", ""),
		SMTPPort: getEnv("SMTP_PORT", ""),
		SMTPUser: getEnv("SMTP_USER", ""),
		SMTPPass: getEnv("SMTP_PASS", ""),

		// PIX
		PixWebhookSecret: getEnv("PIX_WEBHOOK_SECRET", ""),
	}

	// =========================
	// VALIDAÇÃO DE EMAIL
	// =========================
	if cfg.EmailEnabled {
		if cfg.EmailFrom == "" ||
			cfg.SMTPHost == "" ||
			cfg.SMTPPort == "" ||
			cfg.SMTPUser == "" ||
			cfg.SMTPPass == "" {
			log.Fatal("❌ EMAIL_ENABLED=true mas variáveis SMTP incompletas")
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
