package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port          string
	DBUrl         string
	RedisAddr     string
	TgBotToken    string
	OpenAIAPIKey  string
	OpenAIBaseURL string
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("Файл .env не найден, берутся системные переменные")
	}

	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}
	raddr := os.Getenv("REDIS_ADDR")
	if raddr == "" {
		raddr = "redis:6379"
	}
	baseURL := os.Getenv("OPENAI_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	return &Config{
		Port:          port,
		DBUrl:         os.Getenv("DB_URL"),
		RedisAddr:     raddr,
		TgBotToken:    os.Getenv("TG_BOT_TOKEN"),
		OpenAIAPIKey:  os.Getenv("OPENAI_API_KEY"),
		OpenAIBaseURL: baseURL,
	}
}
