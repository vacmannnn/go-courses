package handler

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
)

type Handler struct{ db *sql.DB }

func New(db *sql.DB) *Handler { return &Handler{db: db} }

type createOrderRequest struct {
	Items []struct {
		ProductID int64 `json:"product_id"`
		Quantity  int   `json:"quantity"`
	} `json:"items"`
}

func badRequest(w http.ResponseWriter, message string) { http.Error(w, message, 400) }
func internalError(w http.ResponseWriter, err error) {
	slog.Error("request failed", "error", err)
	http.Error(w, "internal error", 500)
}
func (h *Handler) CreateOrder(
	w http.ResponseWriter, r *http.Request,
) {
	ctx := r.Context()
	var req createOrderRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		badRequest(w, "invalid JSON")
		return
	}
	if len(req.Items) == 0 {
		badRequest(w, "empty order")
		return
	}
	var total int64
	for _, item := range req.Items {
		if item.Quantity <= 0 {
			badRequest(w, "quantity <= 0")
			return
		}
		var price int64
		err = h.db.QueryRowContext(ctx,
			`SELECT price FROM products
             WHERE id = $1`, item.ProductID,
		).Scan(&price)
		if err != nil {
			internalError(w, err)
			return
		}
		total += price * int64(item.Quantity)
	}
	items, err := json.Marshal(req.Items)
	if err != nil {
		internalError(w, err)
		return
	}
	var id int64
	err = h.db.QueryRowContext(ctx,
		`INSERT INTO orders (items, total)
         VALUES ($1, $2) RETURNING id`,
		string(items), total,
	).Scan(&id)
	if err != nil {
		internalError(w, err)
		return
	}
	w.Header().Set("Content-Type",
		"application/json")
	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, `{"order_id":%d}`, id)
}
