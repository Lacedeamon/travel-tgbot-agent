package hotels

import (
	"reflect"
	"strings"
	"testing"
)

func TestMaxPricePerNight(t *testing.T) {
	cases := []struct {
		name string
		p    SearchParams
		want int
	}{
		{"бюджет делится на ночи", SearchParams{Budget: 100000, Nights: 7}, 14285},
		{"нет ночей", SearchParams{Budget: 100000}, 0},
		{"нет бюджета", SearchParams{Nights: 7}, 0},
	}
	for _, c := range cases {
		if got := c.p.MaxPricePerNight(); got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
}

func TestBuildQueryEmpty(t *testing.T) {
	q, args := buildQuery(SearchParams{})
	if strings.Contains(q, "WHERE") {
		t.Errorf("пустые параметры не должны давать WHERE: %s", q)
	}
	if !reflect.DeepEqual(args, []any{defaultLimit}) {
		t.Errorf("args = %v", args)
	}
	if !strings.HasSuffix(q, "LIMIT $1") {
		t.Errorf("query = %s", q)
	}
}

func TestBuildQueryFilters(t *testing.T) {
	q, args := buildQuery(SearchParams{
		Cities:          []string{"Кемер", "Белек"},
		Budget:          100000,
		Nights:          7,
		MinStars:        4,
		MinRating:       4.5,
		MaxBeachMinutes: 10,
		AllInclusive:    true,
		Pool:            true,
		Limit:           3,
	})

	for _, want := range []string{
		"city IN ($1, $2)",
		"price_per_night_rub <= $3",
		"stars >= $4",
		"rating >= $5",
		"beach_minutes <= $6",
		"all_inclusive AND pool",
		"LIMIT $7",
	} {
		if !strings.Contains(q, want) {
			t.Errorf("в запросе нет %q:\n%s", want, q)
		}
	}
	wantArgs := []any{"Кемер", "Белек", 14285, 4, 4.5, 10, 3}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Errorf("args = %v, want %v", args, wantArgs)
	}
}

func TestBuildQueryNoInjection(t *testing.T) {
	q, args := buildQuery(SearchParams{Cities: []string{"x'; DROP TABLE hotels;--"}})
	if strings.Contains(q, "DROP") {
		t.Errorf("значение попало в текст запроса: %s", q)
	}
	if args[0] != "x'; DROP TABLE hotels;--" {
		t.Errorf("args = %v", args)
	}
}

func TestBuildQueryExclude(t *testing.T) {
	q, args := buildQuery(SearchParams{Exclude: []string{"A"}})
	if !strings.Contains(q, "NOT (name = ANY($1))") || len(args) != 2 {
		t.Errorf("q = %s, args = %v", q, args)
	}
}
