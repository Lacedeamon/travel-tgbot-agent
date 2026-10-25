package storage

import (
	"context"
	"log"

	"github.com/redis/go-redis/v9"
)

func InitRedis(addr string) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("Ошибка подключения к Redis: %v", err)
	}

	log.Println("Подключение к Redis успешно установлено")
	return client
}
