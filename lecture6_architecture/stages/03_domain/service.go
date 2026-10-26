package usecase

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/vacmannnn/go-courses/lecture6_architecture/internal/domain"
	"github.com/vacmannnn/go-courses/lecture6_architecture/internal/dto"
)

type Service struct{ db *sql.DB }

func New(db *sql.DB) *Service { return &Service{db: db} }
func (s *Service) CreateOrder(ctx context.Context, input dto.CreateOrderInput) (dto.CreateOrderOutput, error) {
	items := make([]domain.Item, 0, len(input.Items))
	for _, item := range input.Items {
		var price int64
		err := s.db.QueryRowContext(ctx, `SELECT price FROM products WHERE id=$1`, item.ProductID).Scan(&price)
		if err != nil {
			return dto.CreateOrderOutput{}, err
		}
		items = append(items, domain.Item{ProductID: item.ProductID, Quantity: item.Quantity, Price: price})
	}
	order, err := domain.NewOrder(items)
	if err != nil {
		return dto.CreateOrderOutput{}, err
	}
	data, err := json.Marshal(order.Items())
	if err != nil {
		return dto.CreateOrderOutput{}, err
	}
	var id int64
	err = s.db.QueryRowContext(ctx, `INSERT INTO orders(items,total) VALUES($1,$2) RETURNING id`, string(data), order.Total()).Scan(&id)
	if err != nil {
		return dto.CreateOrderOutput{}, err
	}
	return dto.CreateOrderOutput{OrderID: id}, nil
}
