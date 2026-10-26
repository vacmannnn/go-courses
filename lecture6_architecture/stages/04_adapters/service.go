package usecase

import (
	"context"
	"fmt"
	"github.com/vacmannnn/go-courses/lecture6_architecture/internal/adapter"
	"github.com/vacmannnn/go-courses/lecture6_architecture/internal/domain"
	"github.com/vacmannnn/go-courses/lecture6_architecture/internal/dto"
)

// Промежуточное состояние: сценарий импортирует конкретные адаптеры.
type Service struct {
	catalog *adapter.PostgresCatalog
	orders  *adapter.PostgresOrders
}

func New(catalog *adapter.PostgresCatalog, orders *adapter.PostgresOrders) *Service {
	return &Service{catalog: catalog, orders: orders}
}
func (s *Service) CreateOrder(ctx context.Context, input dto.CreateOrderInput) (dto.CreateOrderOutput, error) {
	items := make([]domain.Item, 0, len(input.Items))
	for _, item := range input.Items {
		price, err := s.catalog.Price(ctx, item.ProductID)
		if err != nil {
			return dto.CreateOrderOutput{}, fmt.Errorf("get price: %w", err)
		}
		items = append(items, domain.Item{ProductID: item.ProductID, Quantity: item.Quantity, Price: price})
	}
	order, err := domain.NewOrder(items)
	if err != nil {
		return dto.CreateOrderOutput{}, err
	}
	id, err := s.orders.Save(ctx, order)
	if err != nil {
		return dto.CreateOrderOutput{}, fmt.Errorf("save order: %w", err)
	}
	return dto.CreateOrderOutput{OrderID: id}, nil
}
