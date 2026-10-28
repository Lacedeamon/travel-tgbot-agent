package hotels

import (
	"reflect"
	"strings"
	"testing"
)

func TestExpandWithChildren(t *testing.T) {
	p := SearchParams{WithChildren: true}.expand()
	if !p.AllInclusive || !p.Pool || !p.NearBeach {
		t.Errorf("семейный профиль не развернулся: %+v", p)
	}
	if p.KidsFriendly {
		t.Error("kids_friendly не должен становиться жёстким фильтром")
	}
}

func TestBuildQueryWithChildren(t *testing.T) {
	q, _ := buildQuery(SearchParams{WithChildren: true}.expand())
	for _, want := range []string{"all_inclusive", "pool", "(beachfront OR beach_on_foot)", "ORDER BY kids_friendly DESC, rating"} {
		if !strings.Contains(q, want) {
			t.Errorf("в запросе нет %q:\n%s", want, q)
		}
	}
	q, _ = buildQuery(SearchParams{})
	if strings.Contains(q, "kids_friendly DESC") {
		t.Errorf("без WithChildren сортировка по kids_friendly не нужна:\n%s", q)
	}
}

func TestRelaxStopsWhenEnough(t *testing.T) {
	calls := 0
	res, err := relax(SearchParams{Fitness: true, MinRating: 4.5}, func(p SearchParams) ([]Hotel, error) {
		calls++
		if p.Fitness {
			return nil, nil
		}
		return make([]Hotel, 3), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(res.Relaxed) != 1 || res.Applied.Fitness {
		t.Errorf("calls=%d relaxed=%v applied=%+v", calls, res.Relaxed, res.Applied)
	}
	if res.Applied.MinRating != 4.5 {
		t.Error("рейтинг ослаблен раньше времени")
	}
}

func TestRelaxNothingToRelax(t *testing.T) {
	calls := 0
	res, _ := relax(SearchParams{Cities: []string{"Сиде"}}, func(SearchParams) ([]Hotel, error) {
		calls++
		return nil, nil
	})
	if calls != 1 || len(res.Relaxed) != 0 {
		t.Errorf("calls=%d relaxed=%v", calls, res.Relaxed)
	}
}

func TestRelaxOrderAndKeepsHardFilters(t *testing.T) {
	start := SearchParams{
		Cities: []string{"Сиде"}, Budget: 70000, Nights: 7, Pool: true, PetsAllowed: true,
		MinStars: 4, MinRating: 4.5, MaxBeachMinutes: 5, BeachOnFoot: true,
		AllInclusive: true, Beachfront: true, Spa: true,
	}
	res, _ := relax(start, func(SearchParams) ([]Hotel, error) { return nil, nil })

	if len(res.Relaxed) != len(relaxSteps) {
		t.Fatalf("ожидали %d шагов, получили %v", len(relaxSteps), res.Relaxed)
	}
	a := res.Applied
	if !reflect.DeepEqual(a.Cities, start.Cities) || a.Budget != start.Budget || a.Nights != start.Nights || !a.Pool || !a.PetsAllowed {
		t.Errorf("жёсткие фильтры затронуты: %+v", a)
	}
	if a.MinStars != 3 || a.MinRating != 0 || a.AllInclusive || a.Beachfront || a.Spa || a.MaxBeachMinutes != 0 {
		t.Errorf("мягкие фильтры не ослаблены: %+v", a)
	}
}
