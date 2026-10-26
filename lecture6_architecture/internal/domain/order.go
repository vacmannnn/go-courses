package domain

import (
	"errors"
	"math"
)

var (
	ErrEmptyOrder      = errors.New("empty order")
	ErrInvalidQuantity = errors.New("quantity must be positive")
	ErrInvalidPrice    = errors.New("price must be nonnegative")
	ErrTotalOverflow   = errors.New("order total is too large")
	ErrProductNotFound = errors.New("product not found")
)

type Item struct {
	ProductID int64
	Quantity  int
	Price     int64 // копейки за единицу, полученные из каталога
}

// Order сохраняет собственную копию строк и рассчитанную сумму.
// Поля закрыты: после создания правила нельзя обойти прямой записью.
type Order struct {
	items []Item
	total int64
}

func NewOrder(items []Item) (Order, error) {
	if len(items) == 0 {
		return Order{}, ErrEmptyOrder
	}
	var total int64
	for _, item := range items {
		if item.Quantity <= 0 {
			return Order{}, ErrInvalidQuantity
		}
		if item.Price < 0 {
			return Order{}, ErrInvalidPrice
		}
		quantity := int64(item.Quantity)
		if item.Price != 0 && quantity > (math.MaxInt64-total)/item.Price {
			return Order{}, ErrTotalOverflow
		}
		total += item.Price * quantity
	}
	return Order{items: append([]Item(nil), items...), total: total}, nil
}

func (o Order) Total() int64  { return o.total }
func (o Order) Items() []Item { return append([]Item(nil), o.items...) }
