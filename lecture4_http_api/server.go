package main

import (
	"encoding/json"
	"io"
	"log"
	"mime"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Book struct {
	ID     int    `json:"id"`
	Title  string `json:"title"`
	Author string `json:"author"`
}

// Отдельный тип входных данных: ID назначает сервер, а не клиент.
type BookInput struct {
	Title  string `json:"title"`
	Author string `json:"author"`
}

type APIError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type bookAPI struct {
	mu     sync.Mutex // HTTP обрабатывает запросы в разных горутинах.
	books  map[int]Book
	nextID int
}

func newHandler() http.Handler {
	api := &bookAPI{
		books:  map[int]Book{1: {ID: 1, Title: "The Go Programming Language", Author: "Alan Donovan, Brian Kernighan"}},
		nextID: 2,
	}
	mux := http.NewServeMux()
	// Маршруты с методами и параметрами появились в Go 1.22.
	mux.HandleFunc("GET /books", api.list)
	mux.HandleFunc("GET /books/{id}", api.get)
	mux.HandleFunc("POST /books", api.create)
	mux.HandleFunc("PUT /books/{id}", api.replace)
	mux.HandleFunc("DELETE /books/{id}", api.delete)
	mux.HandleFunc("GET /openapi.json", serveSpec)
	mux.HandleFunc("GET /docs/{$}", serveDocs)
	return logging(mux)
}

// Middleware оборачивает следующий handler. Здесь оно измеряет время запроса.
func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	var response APIError
	response.Error.Code = code
	response.Error.Message = message
	writeJSON(w, status, response)
}

func readInput(w http.ResponseWriter, r *http.Request) (BookInput, bool) {
	var input BookInput
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeError(w, 415, "unsupported_media_type", "Use Content-Type: application/json")
		return input, false
	}
	// Лимит защищает от слишком большого тела запроса.
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, 400, "invalid_json", "Expected a JSON object with title and author (max 1 MiB)")
		return input, false
	}
	// После первого объекта не должно быть второго JSON или мусора.
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeError(w, 400, "invalid_json", "Expected exactly one JSON object")
		return input, false
	}
	input.Title = strings.TrimSpace(input.Title)
	input.Author = strings.TrimSpace(input.Author)
	if input.Title == "" || input.Author == "" {
		writeError(w, 422, "validation_error", "title and author must not be blank")
		return input, false
	}
	return input, true
}

func bookID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		writeError(w, 400, "invalid_id", "id must be a positive integer")
		return 0, false
	}
	return id, true
}

func (api *bookAPI) list(w http.ResponseWriter, r *http.Request) {
	api.mu.Lock()
	books := make([]Book, 0, len(api.books)) // Пустой список — [], а не null.
	for _, book := range api.books {
		books = append(books, book)
	}
	api.mu.Unlock()
	sort.Slice(books, func(i, j int) bool { return books[i].ID < books[j].ID })
	writeJSON(w, http.StatusOK, books)
}

func (api *bookAPI) get(w http.ResponseWriter, r *http.Request) {
	id, ok := bookID(w, r)
	if !ok {
		return
	}
	api.mu.Lock()
	book, exists := api.books[id]
	api.mu.Unlock()
	if !exists {
		writeError(w, 404, "book_not_found", "Book not found")
		return
	}
	writeJSON(w, 200, book)
}

func (api *bookAPI) create(w http.ResponseWriter, r *http.Request) {
	input, ok := readInput(w, r)
	if !ok {
		return
	}
	api.mu.Lock()
	book := Book{ID: api.nextID, Title: input.Title, Author: input.Author}
	api.nextID++
	api.books[book.ID] = book
	api.mu.Unlock()
	w.Header().Set("Location", "/books/"+strconv.Itoa(book.ID))
	writeJSON(w, http.StatusCreated, book)
}

// PUT полностью заменяет существующую книгу. Создание по ID здесь не поддерживается.
func (api *bookAPI) replace(w http.ResponseWriter, r *http.Request) {
	id, ok := bookID(w, r)
	if !ok {
		return
	}
	input, ok := readInput(w, r)
	if !ok {
		return
	}
	api.mu.Lock()
	_, exists := api.books[id]
	book := Book{ID: id, Title: input.Title, Author: input.Author}
	if exists {
		api.books[id] = book
	}
	api.mu.Unlock()
	if !exists {
		writeError(w, 404, "book_not_found", "Book not found")
		return
	}
	writeJSON(w, 200, book)
}

func (api *bookAPI) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := bookID(w, r)
	if !ok {
		return
	}
	api.mu.Lock()
	_, exists := api.books[id]
	delete(api.books, id)
	api.mu.Unlock()
	if !exists {
		writeError(w, 404, "book_not_found", "Book not found")
		return
	}
	w.WriteHeader(http.StatusNoContent) // 204 не содержит тела.
}
