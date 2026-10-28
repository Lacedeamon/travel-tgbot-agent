package hotels

import (
	"context"
	"database/sql"
)

// minResults: сколько отелей считаем достаточным, чтобы не ослаблять фильтры дальше.
const minResults = 3

// Result — выдача поиска и список того, что пришлось ослабить, чтобы её получить.
type Result struct {
	Hotels  []Hotel
	Relaxed []string     // человекочитаемые описания, агент пересказывает их пользователю
	Applied SearchParams // параметры, с которыми получена выдача
}

// relaxStep ослабляет один фильтр; возвращает false, если ослаблять было нечего.
type relaxStep struct {
	desc  string
	apply func(p *SearchParams) bool
}

// relaxSteps идут от самых несущественных ограничений к самым важным.
// Город, бюджет, бассейн и «можно с животными» не ослабляются никогда: без них подборка теряет смысл.
var relaxSteps = []relaxStep{
	{"убраны второстепенные удобства (фитнес, парковка, wifi, кухня, ресторан, спа, трансфер)", func(p *SearchParams) bool {
		changed := p.Fitness || p.FreeParking || p.FreeWifi || p.Kitchen || p.Restaurant || p.Spa || p.Transfer
		p.Fitness, p.FreeParking, p.FreeWifi, p.Kitchen, p.Restaurant, p.Spa, p.Transfer = false, false, false, false, false, false, false
		return changed
	}},
	{"не ограничиваются время и способ добраться до пляжа", func(p *SearchParams) bool {
		changed := p.MaxBeachMinutes > 0 || p.BeachOnFoot
		p.MaxBeachMinutes, p.BeachOnFoot = 0, false
		return changed
	}},
	{"рейтинг отеля не ограничивается", func(p *SearchParams) bool {
		changed := p.MinRating > 0
		p.MinRating = 0
		return changed
	}},
	{"отели не обязательно на первой линии и у пляжа", func(p *SearchParams) bool {
		changed := p.Beachfront || p.NearBeach
		p.Beachfront, p.NearBeach = false, false
		return changed
	}},
	{"допускаются отели на звезду ниже", func(p *SearchParams) bool {
		if p.MinStars <= 1 {
			return false
		}
		p.MinStars--
		return true
	}},
	{"не требуется «всё включено» и бесплатный завтрак", func(p *SearchParams) bool {
		changed := p.AllInclusive || p.FreeBreakfast
		p.AllInclusive, p.FreeBreakfast = false, false
		return changed
	}},
}

// SearchRelaxed ищет отели и, если нашлось меньше minResults, по шагам ослабляет фильтры.
func SearchRelaxed(ctx context.Context, db *sql.DB, p SearchParams) (Result, error) {
	return relax(p.expand(), func(p SearchParams) ([]Hotel, error) {
		return search(ctx, db, p)
	})
}

func relax(p SearchParams, find func(SearchParams) ([]Hotel, error)) (Result, error) {
	want := minResults
	if p.Limit > 0 && p.Limit < want {
		want = p.Limit
	}

	res := Result{Applied: p}
	for i := 0; ; i++ {
		found, err := find(res.Applied)
		if err != nil {
			return res, err
		}
		res.Hotels = found
		if len(found) >= want || i >= len(relaxSteps) {
			return res, nil
		}

		// Пропускаем шаги, которые ничего не меняют: лишний запрос и ложное «ослабили» не нужны.
		for ; i < len(relaxSteps); i++ {
			if relaxSteps[i].apply(&res.Applied) {
				res.Relaxed = append(res.Relaxed, relaxSteps[i].desc)
				break
			}
		}
		if i >= len(relaxSteps) {
			return res, nil
		}
	}
}
