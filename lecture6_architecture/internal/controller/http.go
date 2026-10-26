package controller

import (
	"encoding/json"
	"errors"
	"github.com/vacmannnn/go-courses/lecture6_architecture/internal/domain"
	"github.com/vacmannnn/go-courses/lecture6_architecture/internal/dto"
	"github.com/vacmannnn/go-courses/lecture6_architecture/internal/usecase"
	"io"
	"log/slog"
	"net/http"
)

type Handler struct {
	service *usecase.Service
	logger  *slog.Logger
}

func New(service *usecase.Service, logger *slog.Logger) *Handler { return &Handler{service, logger} }

type createOrderRequest struct {
	Items []struct {
		ProductID int64 `json:"product_id"`
		Quantity  int   `json:"quantity"`
	} `json:"items"`
}
type createOrderResponse struct {
	OrderID int64 `json:"order_id"`
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /orders", h.CreateOrder)
	return mux
}
func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	var req createOrderRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "invalid JSON", 400)
		return
	}
	// В теле должен быть ровно один JSON-документ.
	if err := decoder.Decode(new(any)); err != io.EOF {
		http.Error(w, "invalid JSON", 400)
		return
	}
	input := dto.CreateOrderInput{Items: make([]dto.OrderItemInput, 0, len(req.Items))}
	for _, item := range req.Items {
		input.Items = append(input.Items, dto.OrderItemInput{ProductID: item.ProductID, Quantity: item.Quantity})
	}
	output, err := h.service.CreateOrder(r.Context(), input)
	if err != nil {
		h.writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(createOrderResponse{OrderID: output.OrderID}); err != nil {
		h.logger.Error("write response", "error", err)
	}
}
func (h *Handler) writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrEmptyOrder), errors.Is(err, domain.ErrInvalidQuantity), errors.Is(err, domain.ErrTotalOverflow):
		http.Error(w, "invalid order", http.StatusBadRequest)
	case errors.Is(err, domain.ErrProductNotFound):
		http.Error(w, "product not found", http.StatusNotFound)
	default:
		h.logger.Error("create order", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
