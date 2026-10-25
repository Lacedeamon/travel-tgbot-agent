package storage

import (
	"database/sql"
	_ "embed"
	"log"

	_ "github.com/lib/pq"
)

var initSQL string

func InitDB(dbURL string) *sql.DB {
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Ошибка подключения к БД: %v", err)
	}

	if err = db.Ping(); err != nil {
		log.Fatalf("База данных недоступна: %v", err)
	}

	if _, err = db.Exec(initSQL); err != nil {
		log.Fatalf("Ошибка выполнения init.sql: %v", err)
	}

	log.Println("База данных инициализирована (таблицы и моки проверены)")
	return db
}
