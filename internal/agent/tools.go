package agent

import (
	"context"
	"encoding/json"
	"errors"
	"log"

	"github.com/ostrovok-hackathon-2026/mr-robot/internal/hotels"
	"github.com/sashabaranov/go-openai"
)

// maxToolRounds ограничивает число обращений модели к инструментам за один ответ.
// После последнего раунда инструменты отключаются, и модель обязана ответить текстом.
const maxToolRounds = 4

type chatCompleter interface {
	CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)
}

type hotelSearcher func(ctx context.Context, p hotels.SearchParams) (hotels.Result, error)

// runChat гоняет диалог с моделью, пока она вызывает инструменты, и возвращает итоговый текст.
func runChat(ctx context.Context, client chatCompleter, search hotelSearcher, messages []openai.ChatCompletionMessage) (string, error) {
	tools := []openai.Tool{hotels.Tool()}

	for round := 0; ; round++ {
		req := openai.ChatCompletionRequest{Model: model, Messages: messages}
		if round < maxToolRounds {
			req.Tools = tools
		}

		resp, err := client.CreateChatCompletion(ctx, req)
		if err != nil {
			return "", err
		}
		if len(resp.Choices) == 0 {
			return "", errors.New("пустой ответ модели")
		}

		msg := resp.Choices[0].Message
		if len(msg.ToolCalls) == 0 {
			return msg.Content, nil
		}

		messages = append(messages, msg)
		for _, call := range msg.ToolCalls {
			messages = append(messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				ToolCallID: call.ID,
				Content:    runTool(ctx, search, call),
			})
		}
	}
}

// runTool выполняет один вызов инструмента. Любая ошибка возвращается модели как JSON,
// чтобы она могла уточнить данные у пользователя или объяснить, что пошло не так.
func runTool(ctx context.Context, search hotelSearcher, call openai.ToolCall) string {
	log.Printf("Вызов инструмента %s: %s", call.Function.Name, call.Function.Arguments)

	if call.Function.Name != hotels.ToolName {
		return toolError("неизвестный инструмент " + call.Function.Name)
	}

	params, err := hotels.ParseArgs(call.Function.Arguments)
	if err != nil {
		return toolError(err.Error())
	}

	res, err := search(ctx, params)
	if err != nil {
		log.Printf("Ошибка поиска отелей: %v", err)
		return toolError("поиск временно недоступен")
	}
	return hotels.ResultJSON(res)
}

func toolError(msg string) string {
	b, _ := json.Marshal(map[string]string{"error": msg})
	return string(b)
}
