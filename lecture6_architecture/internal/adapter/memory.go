package adapter

import (
	"context"
	"github.com/vacmannnn/go-courses/lecture6_architecture/internal/domain"
	"sync"
)

type MemoryCatalog struct{ prices map[int64]int64 }

func NewMemoryCatalog(prices map[int64]int64) *MemoryCatalog {
	copied := make(map[int64]int64, len(prices))
	for id, price := range prices {
		copied[id] = price
	}
	return &MemoryCatalog{prices: copied}
}
func (c *MemoryCatalog) Price(ctx context.Context, id int64) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	price, ok := c.prices[id]
	if !ok {
		return 0, domain.ErrProductNotFound
	}
	return price, nil
}

type MemoryOrders struct {
	mu     sync.Mutex
	nextID int64
	orders map[int64]domain.Order
}

func NewMemoryOrders() *MemoryOrders { return &MemoryOrders{orders: make(map[int64]domain.Order)} }
func (r *MemoryOrders) Save(ctx context.Context, order domain.Order) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	r.nextID++
	r.orders[r.nextID] = order
	return r.nextID, nil
}
