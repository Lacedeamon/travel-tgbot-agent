package bot

import (
	"log"
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
		return c.Send("Привет! Я AI-агент Островка. Куда бы вы хотели отправиться?")
	})

	b.Handle(tele.OnText, func(c tele.Context) error {
		c.Notify(tele.Typing)

		// Передаем сообщение и ID пользователя агенту
		reply := aiAgent.ProcessUserMessage(c.Chat().ID, c.Text())

		if len(reply) > 3900 {
			reply = reply[:3900] + "...\n[Ответ обрезан]"
		}

		return c.Send(reply)
	})

	log.Println("Telegram-бот запущен")
	b.Start()
}
