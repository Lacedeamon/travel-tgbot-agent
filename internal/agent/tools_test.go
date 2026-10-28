package agent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ostrovok-hackathon-2026/mr-robot/internal/hotels"
	"github.com/sashabaranov/go-openai"
)

// fakeClient отдаёт заготовленные ответы по очереди и запоминает запросы.
type fakeClient struct {
	replies []openai.ChatCompletionMessage
	reqs    []openai.ChatCompletionRequest
}

func (f *fakeClient) CreateChatCompletion(_ context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	f.reqs = append(f.reqs, req)
	msg := f.replies[len(f.reqs)-1]
	return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: msg}}}, nil
}

func toolCall(args string) openai.ChatCompletionMessage {
	return openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleAssistant,
		ToolCalls: []openai.ToolCall{{
			ID:       "call_1",
			Type:     openai.ToolTypeFunction,
			Function: openai.FunctionCall{Name: hotels.ToolName, Arguments: args},
		}},
	}
}

func text(s string) openai.ChatCompletionMessage {
	return openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: s}
}

func emptySearch(*hotels.SearchParams) hotelSearcher {
	return func(context.Context, hotels.SearchParams) (hotels.Result, error) { return hotels.Result{}, nil }
}

func TestRunChatNoTools(t *testing.T) {
	c := &fakeClient{replies: []openai.ChatCompletionMessage{text("Привет")}}
	got, err := runChat(context.Background(), c, toolbox{search: emptySearch(nil)}, nil)
	if err != nil || got.Text != "Привет" {
		t.Fatalf("got %q, %v", got.Text, err)
	}
	if len(c.reqs) != 1 || len(c.reqs[0].Tools) != 2 {
		t.Errorf("инструмент должен быть передан в первом запросе: %+v", c.reqs)
	}
}

func TestRunChatToolRoundTrip(t *testing.T) {
	c := &fakeClient{replies: []openai.ChatCompletionMessage{
		toolCall(`{"cities":["Кемер"],"nights":7,"hotel_budget_rub":70000,"with_children":true}`),
		text("Вот отели"),
	}}
	var gotParams hotels.SearchParams
	search := func(_ context.Context, p hotels.SearchParams) (hotels.Result, error) {
		gotParams = p
		return hotels.Result{
			Hotels:  []hotels.Hotel{{Name: "Test Resort", City: "Кемер", PricePerNight: 9000}},
			Applied: p,
		}, nil
	}

	got, err := runChat(context.Background(), c, toolbox{search: search}, []openai.ChatCompletionMessage{{Role: "user", Content: "в Кемер с детьми"}})
	if err != nil || got.Text != "Вот отели" {
		t.Fatalf("got %q, %v", got.Text, err)
	}
	if gotParams.Cities[0] != "Кемер" || gotParams.MaxPricePerNight() != 10000 || !gotParams.WithChildren {
		t.Errorf("параметры поиска: %+v", gotParams)
	}

	msgs := c.reqs[1].Messages
	last := msgs[len(msgs)-1]
	prev := msgs[len(msgs)-2]
	if last.Role != openai.ChatMessageRoleTool || last.ToolCallID != "call_1" || !strings.Contains(last.Content, "Test Resort") {
		t.Errorf("результат инструмента не передан модели: %+v", last)
	}
	if len(prev.ToolCalls) != 1 {
		t.Errorf("перед ответом инструмента должно идти сообщение модели с вызовом: %+v", prev)
	}
}

func TestRunChatBadArgsDoNotSearch(t *testing.T) {
	c := &fakeClient{replies: []openai.ChatCompletionMessage{
		toolCall(`{"hotel_budget_rub":50000}`),
		text("А сколько ночей?"),
	}}
	called := false
	search := func(context.Context, hotels.SearchParams) (hotels.Result, error) {
		called = true
		return hotels.Result{}, nil
	}
	if _, err := runChat(context.Background(), c, toolbox{search: search}, nil); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Error("поиск не должен вызываться при ошибке в аргументах")
	}
	msgs := c.reqs[1].Messages
	if got := msgs[len(msgs)-1].Content; !strings.Contains(got, "error") || !strings.Contains(got, "nights") {
		t.Errorf("модель должна получить текст ошибки: %s", got)
	}
}

func TestRunChatSearchError(t *testing.T) {
	c := &fakeClient{replies: []openai.ChatCompletionMessage{toolCall(`{}`), text("Не вышло")}}
	search := func(context.Context, hotels.SearchParams) (hotels.Result, error) {
		return hotels.Result{}, errors.New("connection refused")
	}
	if _, err := runChat(context.Background(), c, toolbox{search: search}, nil); err != nil {
		t.Fatal(err)
	}
	got := c.reqs[1].Messages[len(c.reqs[1].Messages)-1].Content
	if strings.Contains(got, "connection refused") || !strings.Contains(got, "недоступен") {
		t.Errorf("внутренняя ошибка не должна утекать модели: %s", got)
	}
}

func TestRunChatStopsAfterMaxRounds(t *testing.T) {
	replies := make([]openai.ChatCompletionMessage, 0, maxToolRounds+1)
	for i := 0; i < maxToolRounds; i++ {
		replies = append(replies, toolCall(`{}`))
	}
	replies = append(replies, text("Итог"))
	c := &fakeClient{replies: replies}

	got, err := runChat(context.Background(), c, toolbox{search: emptySearch(nil)}, nil)
	if err != nil || got.Text != "Итог" {
		t.Fatalf("got %q, %v", got.Text, err)
	}
	if len(c.reqs) != maxToolRounds+1 {
		t.Fatalf("запросов %d, ожидали %d", len(c.reqs), maxToolRounds+1)
	}
	if len(c.reqs[maxToolRounds].Tools) != 0 {
		t.Error("в последнем запросе инструменты должны быть отключены")
	}
}

func TestRunToolUnknown(t *testing.T) {
	out, _ := runTool(context.Background(), toolbox{search: emptySearch(nil)}, openai.ToolCall{Function: openai.FunctionCall{Name: "drop_tables"}})
	if !strings.Contains(out, "error") {
		t.Errorf("out = %s", out)
	}
}

func bookCall(args string) openai.ChatCompletionMessage {
	m := toolCall(args)
	m.ToolCalls[0].Function.Name = hotels.BookToolName
	return m
}

func finder(h hotels.Hotel, err error) toolbox {
	return toolbox{
		search: emptySearch(nil),
		find:   func(context.Context, string) (hotels.Hotel, error) { return h, err },
	}
}

func TestRunChatBookingStopsLoop(t *testing.T) {
	c := &fakeClient{replies: []openai.ChatCompletionMessage{
		bookCall(`{"hotel_name":"Crystal Paraiso Verde Resort & SPA","check_in":"14 февраля","nights":14,"guests":2}`),
	}}
	box := finder(hotels.Hotel{Name: "Crystal Paraiso Verde Resort & SPA", City: "Белек", PricePerNight: 5657}, nil)

	got, err := runChat(context.Background(), c, box, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Booked == nil || got.Text != "" {
		t.Fatalf("ожидался заказ без текста: %+v", got)
	}
	if got.Booked.Total != 5657*14 || got.Booked.Guests != 2 || got.Booked.CheckIn != "14 февраля" {
		t.Errorf("booking = %+v", got.Booked)
	}
	if len(c.reqs) != 1 {
		t.Errorf("после заказа модель вызываться не должна, запросов: %d", len(c.reqs))
	}
}

func TestRunChatBookingUnknownHotel(t *testing.T) {
	c := &fakeClient{replies: []openai.ChatCompletionMessage{
		bookCall(`{"hotel_name":"Выдуманный","check_in":"1 мая","nights":3,"guests":1}`),
		text("Уточните отель"),
	}}
	got, err := runChat(context.Background(), c, finder(hotels.Hotel{}, hotels.ErrHotelNotFound), nil)
	if err != nil || got.Booked != nil || got.Text != "Уточните отель" {
		t.Fatalf("got %+v, %v", got, err)
	}
	msgs := c.reqs[1].Messages
	if last := msgs[len(msgs)-1].Content; !strings.Contains(last, "error") {
		t.Errorf("модель должна получить ошибку: %s", last)
	}
}

func TestRunChatBookingBadArgs(t *testing.T) {
	c := &fakeClient{replies: []openai.ChatCompletionMessage{
		bookCall(`{"hotel_name":"X","nights":0,"guests":1}`),
		text("Сколько ночей?"),
	}}
	got, err := runChat(context.Background(), c, finder(hotels.Hotel{}, errors.New("не должен вызываться")), nil)
	if err != nil || got.Booked != nil {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestRunChatBookingRequiresCheckIn(t *testing.T) {
	c := &fakeClient{replies: []openai.ChatCompletionMessage{
		bookCall(`{"hotel_name":"X","nights":3,"guests":1}`),
		text("С какого числа выезжаете?"),
	}}
	got, err := runChat(context.Background(), c, finder(hotels.Hotel{Name: "X"}, nil), nil)
	if err != nil || got.Booked != nil {
		t.Fatalf("без даты заезда заказ оформляться не должен: %+v, %v", got, err)
	}
	msgs := c.reqs[1].Messages
	if last := msgs[len(msgs)-1].Content; !strings.Contains(last, "check_in") {
		t.Errorf("модель должна получить ошибку про check_in: %s", last)
	}
}
