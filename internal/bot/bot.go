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
			"Просто напиши мне своими словами, какой отдых хочешь, какой бюджет и кто едет. " +
			"Например: <i>«Хочу к морю в Турции на неделю вдвоем, бюджет 100 тысяч»</i>. " +
			"Я подберу один лучший отель, а когда согласишься, оформлю заказ. Условия можно менять в любой момент.\n\n" +
			"/checkout — показать карточку последнего заказа.\n" +
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
		card, ok := aiAgent.OrderCard(c.Chat().ID)
		if !ok {
			return c.Send(card.Text)
		}
		album := tele.Album{
			&tele.Photo{File: tele.FromURL(card.Photos[0]), Caption: card.Text}, // Текст прикрепляется только к первому фото
			&tele.Photo{File: tele.FromURL(card.Photos[1])},
		}
		return c.SendAlbum(album, &tele.SendOptions{ParseMode: tele.ModeHTML})
	})

	b.Handle(tele.OnText, func(c tele.Context) error {
		c.Notify(tele.Typing)

		res := aiAgent.Chat(c.Chat().ID, c.Text())
		reply := normalizeHTML(res.Text)

		// Подпись к фото ограничена 1024 символами: длинный ответ идёт отдельным сообщением после альбома.
		if len(res.Photos) >= 2 {
			album := make(tele.Album, len(res.Photos))
			for i, url := range res.Photos {
				album[i] = &tele.Photo{File: tele.FromURL(url)}
			}
			caption := len([]rune(reply)) <= captionLimit
			if caption {
				album[0].(*tele.Photo).Caption = reply
			}
			if err := c.SendAlbum(album, &tele.SendOptions{ParseMode: tele.ModeHTML}); err != nil {
				log.Printf("Ошибка отправки фото: %v", err)
			} else if caption {
				return nil
			}
		}
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
		{Text: "checkout", Description: "Карточка последнего заказа"},
	})

	log.Println("🤖 Telegram-бот запущен и готов к работе")
	b.Start()
}

const captionLimit = 1024

// normalizeHTML приводит HTML от LLM к тому подмножеству, которое понимает Telegram.
func normalizeHTML(s string) string {
	s = strings.ReplaceAll(s, "<p>", "")
	s = strings.ReplaceAll(s, "</p>", "\n")
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "&nbsp;", "\u00a0") // Telegram не знает &nbsp;
	s = strings.ReplaceAll(s, "<strong>", "<b>")
	return strings.ReplaceAll(s, "</strong>", "</b>")
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
