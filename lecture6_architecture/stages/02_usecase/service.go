package usecase

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/vacmannnn/go-courses/lecture6_architecture/internal/dto"
)

type Service struct{ db *sql.DB }

func New(db *sql.DB) *Service { return &Service{db: db} }

var ErrEmptyOrder = errors.New("empty order")
var ErrInvalidQuantity = errors.New("invalid quantity")

func (s *Service) CreateOrder(
	ctx context.Context,
	input dto.CreateOrderInput,
) (dto.CreateOrderOutput, error) {
	if len(input.Items) == 0 {
		return dto.CreateOrderOutput{},
			ErrEmptyOrder
	}
	var total int64
	for _, item := range input.Items {
		if item.Quantity <= 0 {
			return dto.CreateOrderOutput{},
				ErrInvalidQuantity
		}
		var price int64
		err := s.db.QueryRowContext(ctx,
			`SELECT price FROM products
             WHERE id = $1`, item.ProductID,
		).Scan(&price)
		if err != nil {
			return dto.CreateOrderOutput{}, err
		}
		total += price * int64(item.Quantity)
	}
	items, err := json.Marshal(input.Items)
	if err != nil {
		return dto.CreateOrderOutput{}, err
	}
	var id int64
	err = s.db.QueryRowContext(ctx,
		`INSERT INTO orders (items, total)
         VALUES ($1, $2) RETURNING id`,
		string(items), total,
	).Scan(&id)
	if err != nil {
		return dto.CreateOrderOutput{}, err
	}
	return dto.CreateOrderOutput{OrderID: id}, nil
}
