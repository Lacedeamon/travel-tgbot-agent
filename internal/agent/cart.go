package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"
)

// TripCart хранит текущее состояние планируемой поездки
type TripCart struct {
	City       string `json:"city"`
	Dates      string `json:"dates"`
	Guests     int    `json:"guests"`
	HotelName  string `json:"hotel_name"`
	TotalPrice int    `json:"total_price"`
	Status     string `json:"status"`  // planning, ready, booked
	Photo1     string `json:"photo_1"` // Ссылка на фасад
	Photo2     string `json:"photo_2"` // Ссылка на номер
}

// Заглушки на случай, если у отеля в базе нет фото.
var placeholderPhotos = [2]string{
	"https://images.unsplash.com/photo-1566073771259-6a8506099945?w=800&q=80",
	"https://images.unsplash.com/photo-1582719478250-c89cae4dc85b?w=800&q=80",
}

func (a *Agent) getCartKey(userID int64) string {
	return fmt.Sprintf("trip_cart:%d", userID)
}

// GetCart возвращает корзину пользователя или пустую структуру, если ее нет
func (a *Agent) GetCart(userID int64) (*TripCart, error) {
	ctx := context.Background()
	val, err := a.rdb.Get(ctx, a.getCartKey(userID)).Result()
	if err != nil {
		return &TripCart{Status: "planning"}, nil
	}

	var cart TripCart
	if err := json.Unmarshal([]byte(val), &cart); err != nil {
		return &TripCart{Status: "planning"}, err
	}
	return &cart, nil
}

// UpdateCart сохраняет обновленную корзину в Redis на 24 часа
func (a *Agent) UpdateCart(userID int64, cart *TripCart) error {
	ctx := context.Background()
	data, err := json.Marshal(cart)
	if err != nil {
		return err
	}
	return a.rdb.Set(ctx, a.getCartKey(userID), data, 24*time.Hour).Err()
}

// ClearCart удаляет корзину
func (a *Agent) ClearCart(userID int64) error {
	ctx := context.Background()
	return a.rdb.Del(ctx, a.getCartKey(userID)).Err()
}

// Checkout генерирует финальный чек и "ссылку" на оплату
func (a *Agent) Checkout(userID int64) string {
	cart, _ := a.GetCart(userID)

	// Если корзина пустая (защита от ложных вызовов)
	if cart.HotelName == "" || cart.TotalPrice == 0 {
		return "В вашей корзине пока нет выбранного отеля. Давайте сначала подберем вариант!"
	}

	summary, _ := renderCheckout(cart)

	// Очищаем корзину и историю после успешного оформления, чтобы начать с чистого листа
	a.ClearCart(userID)
	a.ClearHistory(userID)

	return summary
}

// renderCheckout собирает карточку заказа и возвращает её вместе с номером заказа; ничего не очищает.
func renderCheckout(cart *TripCart) (summary, orderID string) {
	// Генерируем фейковый номер заказа
	orderID = fmt.Sprintf("OST-2026-%d", rand.Intn(90000)+10000)

	summary = fmt.Sprintf("🎉 <b>Бронирование почти завершено!</b>\n\n"+
		"🧾 <b>Номер заказа:</b> %s\n"+
		"🏨 <b>Отель:</b> %s\n"+
		"📍 <b>Локация:</b> %s\n"+
		"📅 <b>Даты:</b> %s\n"+
		"👥 <b>Гостей:</b> %d\n"+
		"💰 <b>Итого к оплате:</b> %d руб.\n\n"+
		"🔗 Оплатить заказ можно по защищенной ссылке:\n"+
		"<a href='https://ostrovok.ru/pay/%s'>Перейти к оплате</a>\n\n"+
		"<i>Спасибо, что доверяете свой отдых Островку!</i> 🌴",
		orderID, cart.HotelName, cart.City, cart.Dates, cart.Guests, cart.TotalPrice, orderID)
	return summary, orderID
}
