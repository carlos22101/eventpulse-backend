package config

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port    string
	Env     string
	DB      DBConfig
	Redis   RedisConfig
	JWT     JWTConfig
	WS      WSConfig
}

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type JWTConfig struct {
	Secret          string
	ExpirationHours int
}

type WSConfig struct {
	MaxMessageSize int64
	PongWait       int
}

func (d DBConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.Name, d.SSLMode,
	)
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, reading from environment variables")
	}

	redisDB, _ := strconv.Atoi(getEnv("REDIS_DB", "0"))
	jwtExp, _ := strconv.Atoi(getEnv("JWT_EXPIRATION_HOURS", "24"))
	wsMaxMsg, _ := strconv.ParseInt(getEnv("WS_MAX_MESSAGE_SIZE", "1024"), 10, 64)
	wsPong, _ := strconv.Atoi(getEnv("WS_PONG_WAIT_SECONDS", "60"))

	return &Config{
		Port: getEnv("PORT", "8080"),
		Env:  getEnv("ENV", "development"),
		DB: DBConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "eventpulse"),
			Password: getEnv("DB_PASSWORD", ""),
			Name:     getEnv("DB_NAME", "eventpulse_db"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       redisDB,
		},
		JWT: JWTConfig{
			Secret:          getEnv("JWT_SECRET", ""),
			ExpirationHours: jwtExp,
		},
		WS: WSConfig{
			MaxMessageSize: wsMaxMsg,
			PongWait:       wsPong,
		},
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	if fallback == "" {
		log.Printf("WARNING: environment variable %s is not set", key)
	}
	return fallback
}
