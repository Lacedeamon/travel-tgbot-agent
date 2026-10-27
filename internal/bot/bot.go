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
		err := aiAgent.ClearHistory(c.Chat().ID)
		if err != nil {
			log.Printf("Ошибка сброса истории для %d: %v", c.Chat().ID, err)
		}
		return c.Send("🗑 <i>Ваша история диалога в Redis полностью очищена.</i> Начнем заново!", &tele.SendOptions{ParseMode: tele.ModeHTML})
	})

	b.Handle(tele.OnText, func(c tele.Context) error {
		c.Notify(tele.Typing)

		reply := aiAgent.ProcessUserMessage(c.Chat().ID, c.Text())
		reply = strings.ReplaceAll(reply, "<p>", "")
		reply = strings.ReplaceAll(reply, "</p>", "\n")
		reply = strings.ReplaceAll(reply, "<br>", "\n")
		reply = strings.ReplaceAll(reply, "<br/>", "\n")
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
