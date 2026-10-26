package dto

type OrderItemInput struct {
	ProductID int64
	Quantity  int
}
type CreateOrderInput struct{ Items []OrderItemInput }
type CreateOrderOutput struct{ OrderID int64 }
