package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/ostrovok-hackathon-2026/mr-robot/config"
	"github.com/ostrovok-hackathon-2026/mr-robot/internal/agent"
	"github.com/ostrovok-hackathon-2026/mr-robot/internal/bot"
	"github.com/ostrovok-hackathon-2026/mr-robot/internal/storage"
)

func main() {
	cfg := config.Load()

	db := storage.InitDB(cfg.DBUrl)
	defer db.Close()

	redisClient := storage.InitRedis(cfg.RedisAddr)
	defer redisClient.Close()

	aiAgent := agent.New(cfg.OpenAIAPIKey, cfg.OpenAIBaseURL, db, redisClient)

	go bot.Start(cfg.TgBotToken, aiAgent)

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	port := ":" + cfg.Port
	fmt.Printf("HTTP-сервер запущен на порту %s\n", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Ошибка сервера: %s\n", err)
	}
}
