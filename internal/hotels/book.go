package hotels

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/sashabaranov/go-openai"
)

// BookToolName — имя функции оформления заказа, которую вызывает LLM.
const BookToolName = "book_hotel"

const maxGuests = 20

// BookArgs — аргументы вызова book_hotel.
type BookArgs struct {
	HotelName string `json:"hotel_name"`
	CheckIn   string `json:"check_in"`
	Nights    int    `json:"nights"`
	Guests    int    `json:"guests"`
}

// BookTool возвращает описание инструмента оформления заказа для запроса к LLM.
func BookTool() openai.Tool {
	return openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name: BookToolName,
			Description: "Оформляет заказ на выбранный отель и показывает пользователю итоговую карточку с ссылкой на оплату. " +
				"Вызывай, как только пользователь выбрал конкретный отель из найденных и согласен оформить заказ " +
				"(«беру этот», «оформляй», «подтверждай»). Не пересказывай после этого выбор и не уговаривай: " +
				"карточку заказа пользователю покажет сам сервис. Если не названа дата заезда, сначала спроси её; число гостей бери из диалога.",
			Parameters: map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"hotel_name", "check_in", "nights", "guests"},
				"properties": map[string]any{
					"hotel_name": map[string]any{"type": "string", "description": "Название отеля в точности как в ответе search_hotels."},
					"check_in":   map[string]any{"type": "string", "description": "Дата заезда так, как её назвал пользователь, например «14 февраля». Обязательна: если пользователь её не называл, сначала спроси, не выдумывай."},
					"nights":     map[string]any{"type": "integer", "description": "Число ночей."},
					"guests":     map[string]any{"type": "integer", "description": "Общее число гостей, взрослые и дети вместе."},
				},
			},
		},
	}
}

// ParseBookArgs разбирает и проверяет аргументы book_hotel.
func ParseBookArgs(raw string) (BookArgs, error) {
	var a BookArgs
	dec := json.NewDecoder(bytes.NewReader([]byte(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&a); err != nil {
		return a, fmt.Errorf("некорректные аргументы: %w", err)
	}
	a.HotelName = strings.TrimSpace(a.HotelName)
	a.CheckIn = strings.TrimSpace(a.CheckIn)
	switch {
	case a.HotelName == "":
		return a, errors.New("hotel_name обязателен")
	case a.CheckIn == "":
		return a, errors.New("check_in обязателен: спроси у пользователя дату заезда")
	case a.Nights < 1 || a.Nights > maxNights:
		return a, fmt.Errorf("nights должно быть от 1 до %d", maxNights)
	case a.Guests < 1 || a.Guests > maxGuests:
		return a, fmt.Errorf("guests должно быть от 1 до %d", maxGuests)
	}
	return a, nil
}

// ErrHotelNotFound возвращается, когда отеля с таким названием нет в базе.
var ErrHotelNotFound = errors.New("отель не найден")

// FindByName ищет отель по точному названию без учёта регистра.
func FindByName(ctx context.Context, db *sql.DB, name string) (Hotel, error) {
	var h Hotel
	err := db.QueryRowContext(ctx,
		"SELECT "+selectColumns+" FROM hotels WHERE lower(name) = lower($1) ORDER BY id LIMIT 1", name).
		Scan(&h.ID, &h.City, &h.Name, &h.Rating, &h.Stars, &h.PricePerNight,
			&h.GoogleURL, &h.OfficialSite, &h.AllInclusive, &h.Pool, &h.Beachfront,
			&h.BeachMinutes, &h.BeachOnFoot, &h.KidsFriendly)
	if errors.Is(err, sql.ErrNoRows) {
		return h, ErrHotelNotFound
	}
	if err != nil {
		return h, fmt.Errorf("find hotel: %w", err)
	}
	return h, nil
}
