package config

import (
	"fmt"
	"os"
)

type Config struct {
	DBUrl      string
	JWTSecret  string
	ServerPort string
}

func Load() *Config {
	return &Config{
		DBUrl:      getEnv("DATABASE_URL", "postgres://barber_user:barber_pass@localhost:5433/barber_db?sslmode=disable"),
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
