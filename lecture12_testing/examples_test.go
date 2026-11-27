package lecture

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestSimple(t *testing.T) {
	total, err := Shipping(3001)
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 {
		t.Errorf("Total want: %d, got: %d", 0, total)
	}
}

func TestShipping(t *testing.T) {
	cases := []struct {
		name    string
		total   int
		want    int
		wantErr bool
	}{
		{"below", 2999, 300, false},
		{"at", 3000, 0, false},
		{"above", 3001, 0, false},
		{"negative", -1, 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Shipping(tc.total)
			if (err != nil) != tc.wantErr || got != tc.want {
				t.Fatalf("Shipping(%d) = (%d, %v), want fee=%d err=%t",
					tc.total, got, err, tc.want, tc.wantErr)
			}
		})
	}
}

func TestQuote(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/quote?total=2999", nil)
	rec := httptest.NewRecorder()
	Quote(rec, req)

	res := rec.Result()
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	if !strings.HasPrefix(res.Header.Get("Content-Type"), "application/json") {
		t.Fatalf("expected JSON, got %q", res.Header.Get("Content-Type"))
	}
	var body struct {
		Fee *int `json:"fee"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Fee == nil || *body.Fee != 300 {
		t.Fatalf("body = %s, want a fee of 300", rec.Body)
	}
}

func TestQuoteInvalidTotal(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/quote?total=broken", nil)
	rec := httptest.NewRecorder()
	Quote(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestQuoteThroughServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(Quote))
	t.Cleanup(srv.Close)

	res, err := srv.Client().Get(srv.URL + "/quote?total=3000")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	var body struct {
		Fee *int `json:"fee"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Fee == nil || *body.Fee != 0 {
		t.Fatal("expected a numeric fee of zero")
	}
}

func FuzzParseSKU(f *testing.F) {
	f.Add("sku-1")
	f.Add("sku-42")
	f.Add("")
	f.Fuzz(func(t *testing.T, input string) {
		id, ok := ParseSKU(input)
		if !ok {
			return
		}
		if id <= 0 {
			t.Fatalf("accepted non-positive id %d from %q", id, input)
		}
		canonical := "sku-" + strconv.Itoa(id)
		got, ok := ParseSKU(canonical)
		if !ok || got != id {
			t.Fatalf("round trip: %q → (%d, %t), want %d", canonical, got, ok, id)
		}
	})
}

func TestParseSKU(t *testing.T) {
	got, ok := ParseSKU("sku-42")
	if !ok || got != 42 {
		t.Fatalf("ParseSKU(%q) = (%d, %t), want (42, true)", "sku-42", got, ok)
	}
}

func TestCSVImplementationsAgree(t *testing.T) {
	for _, items := range [][]string{nil, {"sku-1"}, exampleSKUs()} {
		if a, b := csvConcat(items), csvJoin(items); a != b {
			t.Fatalf("CSV output differs for %d items", len(items))
		}
	}
}

func BenchmarkCSV(b *testing.B) {
	items := exampleSKUs()
	b.Run("concat", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			csvConcat(items)
		}
	})
	b.Run("join", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			csvJoin(items)
		}
	})
}

// показать для бенчмарков трассировку и pprof
// во flamegraph увидеть сравнение двух бенчмарков
// не забыть про benchstat
