package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func request(h http.Handler, method, path, body, contentType string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestBookLifecycle(t *testing.T) {
	h := newHandler()
	book := `{"title":"Learning Go","author":"Jon Bodner"}`
	created := request(h, "POST", "/books", book, "application/json")
	if created.Code != 201 || created.Header().Get("Location") != "/books/2" {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	updated := `{"title":"Updated","author":"Jon Bodner"}`
	for range 2 {
		w := request(h, "PUT", "/books/2", updated, "application/json")
		if w.Code != 200 {
			t.Fatalf("PUT: %d %s", w.Code, w.Body.String())
		}
	}
	w := request(h, "GET", "/books/2", "", "")
	var got Book
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || got.ID != 2 || got.Title != "Updated" {
		t.Fatalf("GET: %d %+v", w.Code, got)
	}
	w = request(h, "DELETE", "/books/2", "", "")
	if w.Code != 204 || w.Body.Len() != 0 {
		t.Fatalf("DELETE: %d %s", w.Code, w.Body.String())
	}
	for _, method := range []string{"GET", "DELETE"} {
		if w := request(h, method, "/books/2", "", ""); w.Code != 404 {
			t.Fatalf("%s: %d", method, w.Code)
		}
	}
}

func TestErrors(t *testing.T) {
	for _, tc := range []struct {
		name, method, path, body, media string
		status                          int
	}{
		{"media", "POST", "/books", `{}`, "text/plain", 415},
		{"broken", "POST", "/books", `{`, "application/json", 400},
		{"unknown field", "POST", "/books", `{"id":7,"title":"Go","author":"A"}`, "application/json", 400},
		{"two objects", "POST", "/books", `{} {}`, "application/json", 400},
		{"null", "POST", "/books", `null`, "application/json", 422},
		{"blank", "POST", "/books", `{"title":" ","author":"A"}`, "application/json", 422},
		{"bad ID", "GET", "/books/nope", "", "", 400},
		{"missing", "GET", "/books/999", "", "", 404},
		{"replace missing", "PUT", "/books/999", `{"title":"Go","author":"A"}`, "application/json", 404},
		{"large body", "POST", "/books", `{"title":"` + strings.Repeat("x", 1<<20) + `","author":"A"}`, "application/json", 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := request(newHandler(), tc.method, tc.path, tc.body, tc.media)
			if w.Code != tc.status {
				t.Fatalf("got %d, want %d: %s", w.Code, tc.status, w.Body.String())
			}
			var got APIError
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got.Error.Code == "" || got.Error.Message == "" {
				t.Fatal("empty error")
			}
		})
	}
}

func TestDocsAndRouting(t *testing.T) {
	h := newHandler()
	for _, path := range []string{"/docs/", "/openapi.json"} {
		w := request(h, "GET", path, "", "")
		if w.Code != 200 || w.Body.Len() == 0 {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
	var spec map[string]any
	w := request(h, "GET", "/openapi.json", "", "")
	if err := json.Unmarshal(w.Body.Bytes(), &spec); err != nil {
		t.Fatal(err)
	}
	if spec["openapi"] != "3.0.3" {
		t.Fatal("invalid OpenAPI version")
	}
	w = request(h, "PATCH", "/books/1", "", "")
	if w.Code != 405 || w.Header().Get("Allow") == "" {
		t.Fatalf("method handling: %d", w.Code)
	}
	if w := request(h, "GET", "/unknown", "", ""); w.Code != 404 {
		t.Fatal(w.Code)
	}
}

func TestConcurrentRequests(t *testing.T) {
	h := newHandler()
	var workers sync.WaitGroup
	for range 20 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			w := request(h, "POST", "/books", `{"title":"Go","author":"A"}`, "application/json")
			if w.Code != 201 {
				t.Errorf("POST: %d", w.Code)
			}
			request(h, "GET", "/books", "", "")
		}()
	}
	workers.Wait()
	w := request(h, "GET", "/books", "", "")
	var books []Book
	if err := json.Unmarshal(w.Body.Bytes(), &books); err != nil {
		t.Fatal(err)
	}
	if len(books) != 21 {
		t.Fatalf("got %d books", len(books))
	}
}
