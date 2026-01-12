package config

import (
	"fmt"
	"log"
	"os"
)

type Config struct {
	DBUrl      string
	JWTSecret  string
	ServerPort string
}

func Load() *Config {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("❌ DATABASE_URL não definida")
	}

	return &Config{
		DBUrl:      dbURL,
		JWTSecret:  getEnv("JWT_SECRET", "changeme"),
		ServerPort: getEnv("SERVER_PORT", "8080"),
	}
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
