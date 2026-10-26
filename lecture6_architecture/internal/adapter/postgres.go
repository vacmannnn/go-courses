package adapter

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/vacmannnn/go-courses/lecture6_architecture/internal/domain"
)

type PostgresCatalog struct{ db *sql.DB }

func NewPostgresCatalog(db *sql.DB) *PostgresCatalog { return &PostgresCatalog{db: db} }
func (c *PostgresCatalog) Price(ctx context.Context, id int64) (int64, error) {
	var price int64
	err := c.db.QueryRowContext(ctx, `SELECT price FROM products WHERE id = $1`, id).Scan(&price)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, domain.ErrProductNotFound
	}
	return price, err
}

type PostgresOrders struct{ db *sql.DB }

func NewPostgresOrders(db *sql.DB) *PostgresOrders { return &PostgresOrders{db: db} }

// Модель хранения локальна адаптеру: domain не содержит JSON-тегов.
type storedItem struct {
	ProductID int64 `json:"product_id"`
	Quantity  int   `json:"quantity"`
	Price     int64 `json:"price"`
}

func (r *PostgresOrders) Save(ctx context.Context, order domain.Order) (int64, error) {
	items := order.Items()
	rows := make([]storedItem, 0, len(items))
	for _, item := range items {
		rows = append(rows, storedItem{item.ProductID, item.Quantity, item.Price})
	}
	data, err := json.Marshal(rows)
	if err != nil {
		return 0, err
	}
	var id int64
	err = r.db.QueryRowContext(ctx,
		`INSERT INTO orders (items, total) VALUES ($1, $2) RETURNING id`,
		string(data), order.Total(),
	).Scan(&id)
	return id, err
}
