package bot

import (
	"log"
	"strings"
	"time"

	"github.com/ostrovok-hackathon-2026/mr-robot/internal/agent"
	tele "gopkg.in/telebot.v3"
)

func Start(token string, aiAgent *agent.Agent) {
	pref := tele.Settings{
		Token:  token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := tele.NewBot(pref)
	if err != nil {
		log.Fatalf("Ошибка запуска бота: %v", err)
		return
	}

	b.Handle("/start", func(c tele.Context) error {
		return c.Send("Привет! Я AI-агент Островка. 🌴\nРасскажи, куда бы ты хотел отправиться, или введи /help.")
	})

	b.Handle("/help", func(c tele.Context) error {
		helpText := "<b>Как со мной общаться:</b>\n\n" +
			"Просто напиши мне своими словами, куда хочешь поехать, какой бюджет и кто едет. " +
			"Например: <i>«Хочу в Сочи на неделю вдвоем, бюджет 100 тысяч»</i>.\n\n" +
			"/reset — очистить историю диалога и начать заново."
		return c.Send(helpText, &tele.SendOptions{ParseMode: tele.ModeHTML})
	})

	b.Handle("/reset", func(c tele.Context) error {
		userID := c.Chat().ID
		aiAgent.ClearHistory(userID)
		aiAgent.ClearCart(userID) // Добавили очистку корзины

		return c.Send("🗑 <i>Ваша корзина и история диалога очищены.</i> Начнем заново!", &tele.SendOptions{ParseMode: tele.ModeHTML})
	})
	b.Handle("/checkout", func(c tele.Context) error {
		aiAgent.UpdateCart(c.Chat().ID, &agent.TripCart{
			City:       "Сочи",
			Dates:      "15.08 - 22.08",
			Guests:     2,
			HotelName:  "Radisson Collection Paradise",
			TotalPrice: 125000,
			Status:     "ready",
			Photo1:     "https://images.unsplash.com/photo-1566073771259-6a8506099945?w=800&q=80",
			Photo2:     "https://images.unsplash.com/photo-1582719478250-c89cae4dc85b?w=800&q=80",
		})

		cart, _ := aiAgent.GetCart(c.Chat().ID)

		summary := aiAgent.Checkout(c.Chat().ID)

		// 4. Формируем красивый альбом из двух фото
		album := tele.Album{
			&tele.Photo{
				File:    tele.FromURL(cart.Photo1),
				Caption: summary, // Текст прикрепляется только к первому фото
			},
			&tele.Photo{
				File: tele.FromURL(cart.Photo2),
			},
		}

		// 5. Отправляем альбом пользователю с параметрами разметки
		return c.SendAlbum(album, &tele.SendOptions{ParseMode: tele.ModeHTML})
	})

	b.Handle(tele.OnText, func(c tele.Context) error {
		c.Notify(tele.Typing)

		res := aiAgent.Chat(c.Chat().ID, c.Text())
		if len(res.Photos) == 2 {
			album := tele.Album{
				&tele.Photo{File: tele.FromURL(res.Photos[0]), Caption: res.Text},
				&tele.Photo{File: tele.FromURL(res.Photos[1])},
			}
			if err := c.SendAlbum(album, &tele.SendOptions{ParseMode: tele.ModeHTML}); err != nil {
				log.Printf("Ошибка отправки карточки заказа: %v", err)
				return c.Send(res.Text, &tele.SendOptions{ParseMode: tele.ModeHTML})
			}
			return nil
		}
		reply := res.Text
		reply = strings.ReplaceAll(reply, "<p>", "")
		reply = strings.ReplaceAll(reply, "</p>", "\n")
		reply = strings.ReplaceAll(reply, "<br>", "\n")
		reply = strings.ReplaceAll(reply, "<br/>", "\n")
		reply = strings.ReplaceAll(reply, "&nbsp;", "\u00a0") // Telegram не знает &nbsp;
		reply = strings.ReplaceAll(reply, "<strong>", "<b>")
		reply = strings.ReplaceAll(reply, "</strong>", "</b>")
		chunks := splitMessage(reply, 4000)

		for _, chunk := range chunks {
			err := c.Send(chunk, &tele.SendOptions{ParseMode: tele.ModeHTML})
			if err != nil {
				log.Printf("Ошибка отправки сообщения (возможно, кривой HTML от LLM): %v", err)
				c.Send(chunk)
			}
		}
		return nil
	})

	b.SetCommands([]tele.Command{
		{Text: "start", Description: "Начать общение"},
		{Text: "help", Description: "Справка"},
		{Text: "reset", Description: "Очистить контекст и корзину"},
		{Text: "checkout", Description: "Загрузить готовое решение"},
	})

	log.Println("🤖 Telegram-бот запущен и готов к работе")
	b.Start()
}

func splitMessage(text string, limit int) []string {
	if len([]rune(text)) <= limit {
		return []string{text}
	}

	var chunks []string
	paragraphs := strings.Split(text, "\n")
	var currentChunk string

	for _, p := range paragraphs {
		if len([]rune(currentChunk+p+"\n")) > limit {
			if currentChunk != "" {
				chunks = append(chunks, strings.TrimSpace(currentChunk))
				currentChunk = p + "\n"
			} else {
				chunks = append(chunks, string([]rune(p)[:limit]))
				currentChunk = string([]rune(p)[limit:]) + "\n"
			}
		} else {
			currentChunk += p + "\n"
		}
	}
	if currentChunk != "" {
		chunks = append(chunks, strings.TrimSpace(currentChunk))
	}
	return chunks
}
