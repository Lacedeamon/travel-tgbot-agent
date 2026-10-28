package agent

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/ostrovok-hackathon-2026/mr-robot/internal/hotels"
	"github.com/redis/go-redis/v9"
	"github.com/sashabaranov/go-openai"
)

//go:embed prompt.txt
var systemPrompt string

const model = "openai/gpt-5.4-nano"

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

// Структура для хранения сообщения в Redis
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

const maxHistoryLength = 10 // Хранить последние 10 сообщений (5 пар вопрос-ответ); вызовы инструментов в историю не попадают

// Reply — ответ агента: текст и, если оформлен заказ, фото для карточки.
type Reply struct {
	Text   string
	Photos []string
}

// ProcessUserMessage возвращает только текст ответа.
func (a *Agent) ProcessUserMessage(userID int64, text string) string {
	return a.Chat(userID, text).Text
}

// Chat обрабатывает сообщение пользователя. Если модель оформила заказ, в ответе карточка заказа с фото.
func (a *Agent) Chat(userID int64, text string) Reply {
	ctx := context.Background()
	redisKey := fmt.Sprintf("chat_history:%d", userID)

	if text == "" {
		return Reply{Text: "Пожалуйста, отправьте текстовый запрос."}
	}

	if systemPrompt == "" {
		log.Println("⚠️ ВНИМАНИЕ: системный промпт пустой!")
	}

	var openaiMessages []openai.ChatCompletionMessage

	openaiMessages = append(openaiMessages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleSystem,
		Content: systemPrompt,
	})

	historyVals, err := a.rdb.LRange(ctx, redisKey, 0, int64(maxHistoryLength-1)).Result()
	if err == nil {

		for i := len(historyVals) - 1; i >= 0; i-- {
			var msg ChatMessage
			if json.Unmarshal([]byte(historyVals[i]), &msg) == nil {
				openaiMessages = append(openaiMessages, openai.ChatCompletionMessage{
					Role:    msg.Role,
					Content: msg.Content,
				})
			}
		}
	}

	openaiMessages = append(openaiMessages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: text,
	})

	res, err := runChat(ctx, a.aiClient, toolbox{search: a.searchHotels, find: a.findHotel}, openaiMessages)
	if err != nil {
		log.Printf("Ошибка OpenAI: %v", err)
		return Reply{Text: "Извините, проблема со связью. Попробуйте позже."}
	}
	if res.Booked != nil {
		return a.checkout(userID, res.Booked)
	}
	reply := res.Text
	photos := shownPhotos(res.Shown, reply)

	// 4. Сохраняем свежий диалог в Redis
	userMsgBytes, _ := json.Marshal(ChatMessage{Role: "user", Content: text})
	assistantMsgBytes, _ := json.Marshal(ChatMessage{Role: "assistant", Content: reply})

	// Пушим в список Redis
	pipe := a.rdb.Pipeline()
	// Голова списка — самое новое, поэтому ответ пушим после вопроса: при чтении (с разворотом) пара идёт «вопрос, ответ».
	pipe.LPush(ctx, redisKey, string(userMsgBytes))
	pipe.LPush(ctx, redisKey, string(assistantMsgBytes))
	// Ограничиваем длину истории, чтобы не забивать память и контекст
	pipe.LTrim(ctx, redisKey, 0, int64(maxHistoryLength-1))
	// Устанавливаем TTL (например, сутки, чтобы мусор не копился)
	pipe.Expire(ctx, redisKey, 24*time.Hour)
	_, _ = pipe.Exec(ctx)

	return Reply{Text: reply, Photos: photos}
}

// checkout кладёт заказ в корзину и формирует карточку; Checkout заодно очищает корзину и историю.
func (a *Agent) checkout(userID int64, b *Booking) Reply {
	dates := fmt.Sprintf("%d ноч.", b.Nights)
	if b.CheckIn != "" {
		dates = fmt.Sprintf("заезд %s, %d ноч.", b.CheckIn, b.Nights)
	}
	photos := b.Hotel.PhotoURLs
	for i := len(photos); i < len(placeholderPhotos); i++ {
		photos = append(photos[:i:i], placeholderPhotos[i])
	}
	err := a.UpdateCart(userID, &TripCart{
		City:       b.Hotel.City,
		Dates:      dates,
		Guests:     b.Guests,
		HotelName:  b.Hotel.Name,
		TotalPrice: b.Total,
		Status:     "ready",
		Photo1:     photos[0],
		Photo2:     photos[1],
	})
	if err != nil {
		log.Printf("Ошибка сохранения корзины: %v", err)
		return Reply{Text: "Не удалось оформить заказ, попробуйте ещё раз."}
	}
	return Reply{Text: a.Checkout(userID), Photos: photos[:2]}
}

// maxReplyPhotos — сколько фото отеля прикладываем к ответу.
const maxReplyPhotos = 3

// shownPhotos возвращает фото отеля из поиска, если ответ модели действительно про него.
func shownPhotos(h *hotels.Hotel, reply string) []string {
	if h == nil || len(h.PhotoURLs) == 0 || !strings.Contains(reply, h.Name) {
		return nil
	}
	if len(h.PhotoURLs) > maxReplyPhotos {
		return h.PhotoURLs[:maxReplyPhotos]
	}
	return h.PhotoURLs
}

func (a *Agent) findHotel(ctx context.Context, name string) (hotels.Hotel, error) {
	return hotels.FindByName(ctx, a.db, name)
}

func (a *Agent) searchHotels(ctx context.Context, p hotels.SearchParams) (hotels.Result, error) {
	return hotels.SearchRelaxed(ctx, a.db, p)
}

// Метод для очистки истории (привяжем к команде /reset)
func (a *Agent) ClearHistory(userID int64) error {
	ctx := context.Background()
	redisKey := fmt.Sprintf("chat_history:%d", userID)
	return a.rdb.Del(ctx, redisKey).Err()
}
