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

type hotelFinder func(ctx context.Context, name string) (hotels.Hotel, error)

// toolbox — то, чем инструменты пользуются для работы с базой.
type toolbox struct {
	search hotelSearcher
	find   hotelFinder
}

// Booking — оформленный заказ; по нему агент собирает карточку и очищает диалог.
type Booking struct {
	Hotel   hotels.Hotel
	CheckIn string
	Nights  int
	Guests  int
	Total   int
}

// chatResult — итог диалога с моделью: либо текст, либо оформленный заказ.
type chatResult struct {
	Text   string
	Booked *Booking
}

// runChat гоняет диалог с моделью, пока она вызывает инструменты, и возвращает итоговый текст.
func runChat(ctx context.Context, client chatCompleter, box toolbox, messages []openai.ChatCompletionMessage) (chatResult, error) {
	tools := []openai.Tool{hotels.Tool(), hotels.BookTool()}

	for round := 0; ; round++ {
		req := openai.ChatCompletionRequest{Model: model, Messages: messages}
		if round < maxToolRounds {
			req.Tools = tools
		}

		resp, err := client.CreateChatCompletion(ctx, req)
		if err != nil {
			return chatResult{}, err
		}
		if len(resp.Choices) == 0 {
			return chatResult{}, errors.New("пустой ответ модели")
		}

		msg := resp.Choices[0].Message
		if len(msg.ToolCalls) == 0 {
			return chatResult{Text: msg.Content}, nil
		}

		messages = append(messages, msg)
		for _, call := range msg.ToolCalls {
			out, booking := runTool(ctx, box, call)
			if booking != nil {
				// Заказ оформлен: карточку покажет сервис, модель больше не нужна.
				return chatResult{Booked: booking}, nil
			}
			messages = append(messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				ToolCallID: call.ID,
				Content:    out,
			})
		}
	}
}

// runTool выполняет один вызов инструмента. Любая ошибка возвращается модели как JSON,
// чтобы она могла уточнить данные у пользователя или объяснить, что пошло не так.
// Второе значение не nil, только если заказ оформлен.
func runTool(ctx context.Context, box toolbox, call openai.ToolCall) (string, *Booking) {
	log.Printf("Вызов инструмента %s: %s", call.Function.Name, call.Function.Arguments)

	switch call.Function.Name {
	case hotels.ToolName:
		return runSearch(ctx, box, call), nil
	case hotels.BookToolName:
		return runBook(ctx, box, call)
	}
	return toolError("неизвестный инструмент " + call.Function.Name), nil
}

func runSearch(ctx context.Context, box toolbox, call openai.ToolCall) string {
	params, err := hotels.ParseArgs(call.Function.Arguments)
	if err != nil {
		return toolError(err.Error())
	}

	res, err := box.search(ctx, params)
	if err != nil {
		log.Printf("Ошибка поиска отелей: %v", err)
		return toolError("поиск временно недоступен")
	}
	return hotels.ResultJSON(res)
}

func runBook(ctx context.Context, box toolbox, call openai.ToolCall) (string, *Booking) {
	args, err := hotels.ParseBookArgs(call.Function.Arguments)
	if err != nil {
		return toolError(err.Error()), nil
	}

	h, err := box.find(ctx, args.HotelName)
	if errors.Is(err, hotels.ErrHotelNotFound) {
		return toolError("отеля с таким названием нет, используй название из search_hotels"), nil
	}
	if err != nil {
		log.Printf("Ошибка поиска отеля для заказа: %v", err)
		return toolError("оформление временно недоступно"), nil
	}

	return "", &Booking{
		Hotel:   h,
		CheckIn: args.CheckIn,
		Nights:  args.Nights,
		Guests:  args.Guests,
		Total:   h.PricePerNight * args.Nights,
	}
}

func toolError(msg string) string {
	b, _ := json.Marshal(map[string]string{"error": msg})
	return string(b)
}
