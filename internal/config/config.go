package config

import (
	"fmt"
	"log"
	"os"
)

type Config struct {
	// =========================
	// CORE
	// =========================
	DBUrl      string
	JWTSecret  string
	ServerPort string

	// =========================
	// EMAIL (BREVO SMTP)
	// =========================
	EmailEnabled bool
	EmailFrom    string // ⚠️ EMAIL PURO (ex: corteon@gmail.com)

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

	cfg := &Config{
		// CORE
		DBUrl:      dbURL,
		JWTSecret:  getEnv("JWT_SECRET", "changeme"),
		ServerPort: getEnv("SERVER_PORT", "8080"),

		// EMAIL
		EmailEnabled: getEnv("EMAIL_ENABLED", "false") == "true",
		EmailFrom:    getEnv("EMAIL_FROM", ""), // 🔴 obrigatório se EmailEnabled=true

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

	return cfg
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func (c *Config) Addr() string {
	return fmt.Sprintf(":%s", c.ServerPort)
}
