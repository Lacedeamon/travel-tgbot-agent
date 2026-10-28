package hotels

import (
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
)

func TestParseArgsOK(t *testing.T) {
	p, err := ParseArgs(`{"cities":["кемер"," Белек"],"nights":7,"hotel_budget_rub":70000,"min_stars":4,"with_children":true,"airport_transfer":true}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Cities) != 2 || p.Cities[0] != "Кемер" || p.Cities[1] != "Белек" {
		t.Errorf("города не нормализованы: %v", p.Cities)
	}
	if p.Budget != 70000 || p.Nights != 7 || p.MinStars != 4 || !p.WithChildren || !p.Transfer {
		t.Errorf("params = %+v", p)
	}
	if p.MaxPricePerNight() != 10000 {
		t.Errorf("max price = %d", p.MaxPricePerNight())
	}
}

func TestParseArgsEmpty(t *testing.T) {
	if _, err := ParseArgs(`{}`); err != nil {
		t.Errorf("пустой вызов допустим: %v", err)
	}
}

func TestParseArgsErrors(t *testing.T) {
	cases := map[string]string{
		"бюджет без ночей":      `{"hotel_budget_rub":50000}`,
		"ночи без бюджета":      `{"nights":5}`,
		"неизвестный город":     `{"cities":["Дубай"]}`,
		"звёзды вне диапазона":  `{"min_stars":6}`,
		"рейтинг вне диапазона": `{"min_rating":7}`,
		"слишком много ночей":   `{"nights":100,"hotel_budget_rub":1000}`,
		"отрицательный бюджет":  `{"nights":3,"hotel_budget_rub":-5}`,
		"лишнее поле":           `{"kids_friendly":true}`,
		"не JSON":               `Кемер`,
	}
	for name, raw := range cases {
		if _, err := ParseArgs(raw); err == nil {
			t.Errorf("%s: ожидали ошибку для %s", name, raw)
		}
	}
}

func TestToolSchemaIsValidJSON(t *testing.T) {
	tool := Tool()
	if tool.Function.Name != ToolName {
		t.Errorf("name = %s", tool.Function.Name)
	}
	b, err := json.Marshal(tool.Function.Parameters)
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties map[string]struct {
			Enum  []string `json:"enum"`
			Items struct {
				Enum []string `json:"enum"`
			} `json:"items"`
		} `json:"properties"`
	}
	if err = json.Unmarshal(b, &schema); err != nil {
		t.Fatal(err)
	}
	if got := schema.Properties["cities"].Items.Enum; len(got) != len(Cities) {
		t.Errorf("enum городов = %v", got)
	}
	for _, hidden := range []string{"kids_friendly", "near_beach", "limit"} {
		if _, ok := schema.Properties[hidden]; ok {
			t.Errorf("параметр %s не должен быть в схеме", hidden)
		}
	}
}

// Каждое поле toolArgs должно быть описано в схеме и наоборот.
func TestToolSchemaMatchesArgs(t *testing.T) {
	b, _ := json.Marshal(Tool().Function.Parameters)
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(b, &schema); err != nil {
		t.Fatal(err)
	}
	var args map[string]any
	ab, _ := json.Marshal(toolArgs{})
	_ = json.Unmarshal(ab, &args)

	for k := range args {
		if _, ok := schema.Properties[k]; !ok {
			t.Errorf("поле %s есть в toolArgs, но нет в схеме", k)
		}
	}
	for k := range schema.Properties {
		if _, ok := args[k]; !ok {
			t.Errorf("параметр %s есть в схеме, но нет в toolArgs", k)
		}
	}
}

func TestResultJSON(t *testing.T) {
	r := Result{
		Hotels: []Hotel{{
			Name: "Test Resort", City: "Кемер", PricePerNight: 10000,
			Rating:       sql.NullFloat64{Float64: 4.5, Valid: true},
			BeachMinutes: sql.NullInt16{Int16: 5, Valid: true},
			BeachOnFoot:  sql.NullBool{Bool: true, Valid: true},
		}},
		Relaxed: []string{"рейтинг отеля не ограничивается"},
		Applied: SearchParams{Nights: 7, WithChildren: true},
	}
	var got toolResult
	if err := json.Unmarshal([]byte(ResultJSON(r)), &got); err != nil {
		t.Fatal(err)
	}
	if got.Found != 1 || got.Hotels[0].TotalPrice != 70000 || len(got.Relaxed) != 1 {
		t.Errorf("got = %+v", got)
	}
	if got.Hotels[0].Stars != nil {
		t.Error("stars без значения не должны попадать в ответ")
	}
	if !strings.Contains(got.Note, "marked_for_kids") {
		t.Errorf("нет пояснения про отметку для детей: %q", got.Note)
	}
}

func TestResultJSONEmpty(t *testing.T) {
	out := ResultJSON(Result{})
	if !strings.Contains(out, `"hotels":[]`) || !strings.Contains(out, "Ничего не найдено") {
		t.Errorf("out = %s", out)
	}
}
