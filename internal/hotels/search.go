package hotels

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

const defaultLimit = 5

// SearchParams описывает запрос на подбор отелей. Нулевое значение поля означает «не фильтровать».
type SearchParams struct {
	Cities []string

	Budget int
	Nights int

	MinStars        int
	MinRating       float64
	MaxBeachMinutes int // минуты до пляжа; отели без этого значения не проходят

	// WithChildren разворачивается в семейный профиль (всё включено, бассейн, пляж рядом),
	// а отели с явной отметкой «подходит для детей» идут в выдаче первыми. Сам по себе kids_friendly
	// как жёсткий фильтр не годится: он стоит у единиц отелей.
	WithChildren bool
	NearBeach    bool // первая линия или пляж пешком

	AllInclusive  bool
	FreeBreakfast bool
	Pool          bool
	Beachfront    bool
	BeachOnFoot   bool
	KidsFriendly  bool
	PetsAllowed   bool
	Transfer      bool
	Spa           bool
	Fitness       bool
	FreeParking   bool
	FreeWifi      bool
	Kitchen       bool
	Restaurant    bool

	Limit int
}

type Hotel struct {
	ID            int
	City          string
	Name          string
	Rating        sql.NullFloat64
	Stars         sql.NullInt16
	PricePerNight int
	GoogleURL     sql.NullString
	OfficialSite  sql.NullString
	AllInclusive  bool
	Pool          bool
	Beachfront    bool
	BeachMinutes  sql.NullInt16
	BeachOnFoot   sql.NullBool
	KidsFriendly  bool
}

// MaxPricePerNight возвращает потолок цены за ночь; 0, если бюджет или число ночей не заданы.
func (p SearchParams) MaxPricePerNight() int {
	if p.Budget <= 0 || p.Nights <= 0 {
		return 0
	}
	return p.Budget / p.Nights
}

// boolFilters связывает флаги запроса с колонками таблицы hotels.
func (p SearchParams) boolFilters() []struct {
	on     bool
	column string
} {
	return []struct {
		on     bool
		column string
	}{
		{p.AllInclusive, "all_inclusive"},
		{p.FreeBreakfast, "free_breakfast"},
		{p.Pool, "pool"},
		{p.Beachfront, "beachfront"},
		{p.BeachOnFoot, "beach_on_foot"},
		{p.KidsFriendly, "kids_friendly"},
		{p.PetsAllowed, "pets_allowed"},
		{p.Transfer, "airport_transfer"},
		{p.Spa, "spa"},
		{p.Fitness, "fitness"},
		{p.FreeParking, "free_parking"},
		{p.FreeWifi, "free_wifi"},
		{p.Kitchen, "kitchen"},
		{p.Restaurant, "restaurant"},
	}
}

const selectColumns = `id, city, name, rating, stars, price_per_night_rub, google_url, official_site,
	all_inclusive, pool, beachfront, beach_minutes, beach_on_foot, kids_friendly`

// buildQuery собирает SQL и аргументы; значения подставляются только через плейсхолдеры.
func buildQuery(p SearchParams) (string, []any) {
	var (
		conds []string
		args  []any
	)
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if len(p.Cities) > 0 {
		marks := make([]string, len(p.Cities))
		for i, c := range p.Cities {
			marks[i] = arg(c)
		}
		conds = append(conds, "city IN ("+strings.Join(marks, ", ")+")")
	}
	if max := p.MaxPricePerNight(); max > 0 {
		conds = append(conds, "price_per_night_rub <= "+arg(max))
	}
	if p.MinStars > 0 {
		conds = append(conds, "stars >= "+arg(p.MinStars))
	}
	if p.MinRating > 0 {
		conds = append(conds, "rating >= "+arg(p.MinRating))
	}
	if p.MaxBeachMinutes > 0 {
		conds = append(conds, "beach_minutes <= "+arg(p.MaxBeachMinutes))
	}
	if p.NearBeach {
		conds = append(conds, "(beachfront OR beach_on_foot)")
	}
	for _, f := range p.boolFilters() {
		if f.on {
			conds = append(conds, f.column)
		}
	}

	q := "SELECT " + selectColumns + " FROM hotels"
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}

	limit := p.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	order := ""
	if p.WithChildren {
		order = "kids_friendly DESC, "
	}
	q += " ORDER BY " + order + "rating DESC NULLS LAST, price_per_night_rub ASC, id LIMIT " + arg(limit)
	return q, args
}

// expand превращает высокоуровневые пожелания в конкретные фильтры.
func (p SearchParams) expand() SearchParams {
	if p.WithChildren {
		p.AllInclusive = true
		p.Pool = true
		p.NearBeach = true
	}
	return p
}

// SearchHotels возвращает отели, подходящие под все заданные фильтры, лучшие по рейтингу первыми.
func SearchHotels(ctx context.Context, db *sql.DB, p SearchParams) ([]Hotel, error) {
	return search(ctx, db, p.expand())
}

// search выполняет запрос как есть, без разворачивания WithChildren.
func search(ctx context.Context, db *sql.DB, p SearchParams) ([]Hotel, error) {
	q, args := buildQuery(p)
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("search hotels: %w", err)
	}
	defer rows.Close()

	var res []Hotel
	for rows.Next() {
		var h Hotel
		if err = rows.Scan(&h.ID, &h.City, &h.Name, &h.Rating, &h.Stars, &h.PricePerNight,
			&h.GoogleURL, &h.OfficialSite, &h.AllInclusive, &h.Pool, &h.Beachfront,
			&h.BeachMinutes, &h.BeachOnFoot, &h.KidsFriendly); err != nil {
			return nil, fmt.Errorf("scan hotel: %w", err)
		}
		res = append(res, h)
	}
	return res, rows.Err()
}
