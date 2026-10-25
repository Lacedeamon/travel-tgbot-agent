package agent

import (
	"context"
	"database/sql"
	"log"

	"github.com/redis/go-redis/v9"
	"github.com/sashabaranov/go-openai"
)

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

	resp, err := a.aiClient.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT4oMini,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: "Ты полезный AI-турагент. Не галлюцинируй и перепроверяй свои ответы.",
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
