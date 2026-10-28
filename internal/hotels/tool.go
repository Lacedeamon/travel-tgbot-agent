package hotels

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sashabaranov/go-openai"
)

// ToolName — имя функции, которую вызывает LLM.
const ToolName = "search_hotels"

// Cities — города, которые есть в базе; это же enum в схеме инструмента.
var Cities = []string{"Анталья", "Кемер", "Белек", "Аланья", "Сиде"}

const (
	maxNights = 60
	maxStars  = 5
)

// toolArgs — аргументы вызова search_hotels. Нужные Go-параметры (NearBeach, Limit, KidsFriendly)
// модели намеренно не показываются: профиль «с детьми» разворачивает Go, а не модель.
type toolArgs struct {
	Cities          []string `json:"cities"`
	Nights          int      `json:"nights"`
	HotelBudgetRub  int      `json:"hotel_budget_rub"`
	MinStars        int      `json:"min_stars"`
	MinRating       float64  `json:"min_rating"`
	MaxBeachMinutes int      `json:"max_beach_minutes"`
	WithChildren    bool     `json:"with_children"`
	AllInclusive    bool     `json:"all_inclusive"`
	FreeBreakfast   bool     `json:"free_breakfast"`
	Pool            bool     `json:"pool"`
	Beachfront      bool     `json:"beachfront"`
	BeachOnFoot     bool     `json:"beach_on_foot"`
	PetsAllowed     bool     `json:"pets_allowed"`
	AirportTransfer bool     `json:"airport_transfer"`
	Spa             bool     `json:"spa"`
	Fitness         bool     `json:"fitness"`
	FreeParking     bool     `json:"free_parking"`
	FreeWifi        bool     `json:"free_wifi"`
	Kitchen         bool     `json:"kitchen"`
	Restaurant      bool     `json:"restaurant"`
}

// Tool возвращает описание инструмента для запроса к LLM.
func Tool() openai.Tool {
	cities := make([]any, len(Cities))
	for i, c := range Cities {
		cities[i] = c
	}
	flag := func(desc string) map[string]any {
		return map[string]any{"type": "boolean", "description": desc}
	}

	return openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name: ToolName,
			Description: "Подбирает отели из базы по условиям пользователя. Возвращает до 5 отелей, лучшие по рейтингу первыми, " +
				"и список фильтров, которые пришлось ослабить, чтобы найти варианты. " +
				"Передавай только то, что пользователь назвал или что прямо следует из его слов; остальные параметры не указывай. " +
				"Вызывай инструмент, как только известны хотя бы город и примерный бюджет с числом ночей. " +
				"Никогда не называй отели, которых нет в ответе инструмента.",
			Parameters: map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"cities": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string", "enum": cities},
						"description": "Города поиска. Если пользователь не назвал город, укажи все подходящие под его пожелания или не передавай параметр.",
					},
					"nights": map[string]any{
						"type":        "integer",
						"minimum":     1,
						"maximum":     maxNights,
						"description": "Число ночей. Передавай только вместе с hotel_budget_rub.",
					},
					"hotel_budget_rub": map[string]any{
						"type":        "integer",
						"minimum":     1,
						"description": "Часть бюджета, отведённая на проживание за всё время, в рублях. Бюджет пользователя назван на всю поездку, поэтому из него нужно вычесть примерные расходы на дорогу и питание, а не передавать его целиком. Передавай только вместе с nights.",
					},
					"min_stars": map[string]any{
						"type":        "integer",
						"minimum":     1,
						"maximum":     maxStars,
						"description": "Минимальное число звёзд отеля.",
					},
					"min_rating": map[string]any{
						"type":        "number",
						"minimum":     0,
						"maximum":     5,
						"description": "Минимальный рейтинг отеля по отзывам (шкала 0-5).",
					},
					"max_beach_minutes": map[string]any{
						"type":        "integer",
						"minimum":     1,
						"description": "Максимум минут до ближайшего пляжа.",
					},
					"with_children":    flag("Пользователь едет с детьми. Включает семейный профиль: «всё включено», бассейн и пляж рядом, отели с отметкой для детей идут первыми. Не дублируй эти фильтры отдельными параметрами."),
					"all_inclusive":    flag("Нужен тариф «всё включено»."),
					"free_breakfast":   flag("Нужен бесплатный завтрак."),
					"pool":             flag("Нужен бассейн."),
					"beachfront":       flag("Нужна первая линия: отель на берегу или с выходом на пляж."),
					"beach_on_foot":    flag("До пляжа нужно идти пешком."),
					"pets_allowed":     flag("Можно с домашними животными."),
					"airport_transfer": flag("Нужен трансфер из аэропорта."),
					"spa":              flag("Нужен спа."),
					"fitness":          flag("Нужен фитнес-зал."),
					"free_parking":     flag("Нужна бесплатная парковка."),
					"free_wifi":        flag("Нужен бесплатный wifi."),
					"kitchen":          flag("Нужна кухня или мини-кухня в номере."),
					"restaurant":       flag("Нужен ресторан в отеле."),
				},
			},
		},
	}
}

// ParseArgs разбирает JSON аргументов, пришедший от модели, и проверяет его.
// Ошибка написана так, чтобы её можно было вернуть модели как результат вызова инструмента.
func ParseArgs(raw string) (SearchParams, error) {
	var a toolArgs
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&a); err != nil {
		return SearchParams{}, fmt.Errorf("некорректные аргументы: %w", err)
	}

	if (a.HotelBudgetRub > 0) != (a.Nights > 0) {
		return SearchParams{}, fmt.Errorf("hotel_budget_rub и nights передаются только вместе: уточни у пользователя недостающее")
	}
	if a.HotelBudgetRub < 0 || a.Nights < 0 || a.Nights > maxNights {
		return SearchParams{}, fmt.Errorf("nights должно быть от 1 до %d, hotel_budget_rub положительным", maxNights)
	}
	if a.MinStars < 0 || a.MinStars > maxStars {
		return SearchParams{}, fmt.Errorf("min_stars должно быть от 1 до %d", maxStars)
	}
	if a.MinRating < 0 || a.MinRating > 5 {
		return SearchParams{}, fmt.Errorf("min_rating должно быть от 0 до 5")
	}
	if a.MaxBeachMinutes < 0 {
		return SearchParams{}, fmt.Errorf("max_beach_minutes не может быть отрицательным")
	}

	cities := make([]string, 0, len(a.Cities))
	for _, c := range a.Cities {
		city, ok := normalizeCity(c)
		if !ok {
			return SearchParams{}, fmt.Errorf("города %q нет в базе, доступны: %s", c, strings.Join(Cities, ", "))
		}
		cities = append(cities, city)
	}

	return SearchParams{
		Cities:          cities,
		Budget:          a.HotelBudgetRub,
		Nights:          a.Nights,
		MinStars:        a.MinStars,
		MinRating:       a.MinRating,
		MaxBeachMinutes: a.MaxBeachMinutes,
		WithChildren:    a.WithChildren,
		AllInclusive:    a.AllInclusive,
		FreeBreakfast:   a.FreeBreakfast,
		Pool:            a.Pool,
		Beachfront:      a.Beachfront,
		BeachOnFoot:     a.BeachOnFoot,
		PetsAllowed:     a.PetsAllowed,
		Transfer:        a.AirportTransfer,
		Spa:             a.Spa,
		Fitness:         a.Fitness,
		FreeParking:     a.FreeParking,
		FreeWifi:        a.FreeWifi,
		Kitchen:         a.Kitchen,
		Restaurant:      a.Restaurant,
	}, nil
}

func normalizeCity(s string) (string, bool) {
	s = strings.TrimSpace(s)
	for _, c := range Cities {
		if strings.EqualFold(c, s) {
			return c, true
		}
	}
	return "", false
}

type toolHotel struct {
	Name            string   `json:"name"`
	City            string   `json:"city"`
	Rating          *float64 `json:"rating,omitempty"`
	Stars           *int     `json:"stars,omitempty"`
	PricePerNight   int      `json:"price_per_night_rub"`
	TotalPrice      int      `json:"total_price_rub,omitempty"`
	AllInclusive    bool     `json:"all_inclusive"`
	Pool            bool     `json:"pool"`
	Beachfront      bool     `json:"beachfront"`
	BeachMinutes    *int     `json:"beach_minutes,omitempty"`
	BeachOnFoot     *bool    `json:"beach_on_foot,omitempty"`
	MarkedForKids   bool     `json:"marked_for_kids"`
	OfficialSiteURL string   `json:"official_site,omitempty"`
}

type toolResult struct {
	Found   int         `json:"found"`
	Relaxed []string    `json:"relaxed_filters,omitempty"`
	Note    string      `json:"note,omitempty"`
	Hotels  []toolHotel `json:"hotels"`
}

// ResultJSON сериализует выдачу в ответ инструмента. Цены даны за ночь и за весь срок,
// чтобы модель не считала сама.
func ResultJSON(r Result) string {
	out := toolResult{Found: len(r.Hotels), Relaxed: r.Relaxed, Hotels: []toolHotel{}}

	if len(r.Hotels) == 0 {
		out.Note = "Ничего не найдено даже после ослабления фильтров. Честно скажи об этом и предложи увеличить бюджет или выбрать другой город."
	} else if r.Applied.WithChildren {
		out.Note = "Отметка marked_for_kids есть не у всех отелей: её отсутствие не значит, что отель плох для детей, но и подтверждения нет."
	}

	for _, h := range r.Hotels {
		th := toolHotel{
			Name:          h.Name,
			City:          h.City,
			PricePerNight: h.PricePerNight,
			AllInclusive:  h.AllInclusive,
			Pool:          h.Pool,
			Beachfront:    h.Beachfront,
			MarkedForKids: h.KidsFriendly,
		}
		if r.Applied.Nights > 0 {
			th.TotalPrice = h.PricePerNight * r.Applied.Nights
		}
		if h.Rating.Valid {
			v := h.Rating.Float64
			th.Rating = &v
		}
		if h.Stars.Valid {
			v := int(h.Stars.Int16)
			th.Stars = &v
		}
		if h.BeachMinutes.Valid {
			v := int(h.BeachMinutes.Int16)
			th.BeachMinutes = &v
		}
		if h.BeachOnFoot.Valid {
			v := h.BeachOnFoot.Bool
			th.BeachOnFoot = &v
		}
		if h.OfficialSite.Valid {
			th.OfficialSiteURL = h.OfficialSite.String
		}
		out.Hotels = append(out.Hotels, th)
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(out)
	return strings.TrimSpace(buf.String())
}
