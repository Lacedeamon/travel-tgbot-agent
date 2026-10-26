package agent

import (
	"context"
	"database/sql"
	_ "embed"
	"log"

	"github.com/redis/go-redis/v9"
	"github.com/sashabaranov/go-openai"
)

//go:embed promt.txt
var systemPromt string

type Agent struct {
	aiClient *openai.Client
	db       *sql.DB
	rdb      *redis.Client
}

func New(apiKey, baseURL string, db *sql.DB, rdb *redis.Client) *Agent {
	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = baseURL
	return &Agent{
		aiClient: openai.NewClientWithConfig(cfg),
		db:       db,
		rdb:      rdb,
	}
}

func (a *Agent) ProcessUserMessage(userID int64, text string) string {
	if text == "" {
		return "Пожалуйста, отправьте текстовый запрос."
	}

	if systemPromt == "" {
		log.Println("⚠️ ВНИМАНИЕ: системный промпт пустой! Убедитесь, что prompt.txt не пуст и лежит в одной папке с agent.go")
	}

	resp, err := a.aiClient.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: "openai/gpt-5.4-nano",
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: systemPromt,
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: text,
				},
			},
		},
	)

	if err != nil {
		log.Printf("Ошибка OpenAI: %v", err)
		return "Извините, проблема со связью. Попробуйте позже."
	}

	return resp.Choices[0].Message.Content
}
