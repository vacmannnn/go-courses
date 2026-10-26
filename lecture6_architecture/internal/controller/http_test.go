package controller

import (
	"encoding/json"
	"github.com/vacmannnn/go-courses/lecture6_architecture/internal/adapter"
	"github.com/vacmannnn/go-courses/lecture6_architecture/internal/usecase"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPBoundary(t *testing.T) {
	h := New(usecase.New(adapter.NewMemoryCatalog(map[int64]int64{42: 15000}), adapter.NewMemoryOrders()), slog.New(slog.NewTextHandler(io.Discard, nil))).Routes()
	for _, tc := range []struct {
		name, body string
		status     int
	}{
		{"valid", `{"items":[{"product_id":42,"quantity":2}]}`, 201},
		{"broken JSON", `{`, 400}, {"empty", `{"items":[]}`, 400},
		{"quantity", `{"items":[{"product_id":42,"quantity":0}]}`, 400},
		{"missing product", `{"items":[{"product_id":999,"quantity":1}]}`, 404},
		{"second document", `{"items":[]}{}`, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest("POST", "/orders", strings.NewReader(tc.body)))
			if w.Code != tc.status {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			if tc.status == 201 {
				var body createOrderResponse
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body.OrderID <= 0 {
					t.Fatalf("response=%s", w.Body.String())
				}
			}
		})
	}
}
