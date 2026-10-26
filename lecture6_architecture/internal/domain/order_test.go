package domain

import (
	"errors"
	"math"
	"testing"
)

func TestOrderRules(t *testing.T) {
	cases := []struct {
		name  string
		items []Item
		want  error
	}{
		{"empty", nil, ErrEmptyOrder},
		{"zero quantity", []Item{{Quantity: 0, Price: 100}}, ErrInvalidQuantity},
		{"negative quantity", []Item{{Quantity: -1, Price: 100}}, ErrInvalidQuantity},
		{"negative price", []Item{{Quantity: 1, Price: -1}}, ErrInvalidPrice},
		{"multiplication overflow", []Item{{Quantity: 2, Price: math.MaxInt64}}, ErrTotalOverflow},
		{"addition overflow", []Item{{Quantity: 1, Price: math.MaxInt64}, {Quantity: 1, Price: 1}}, ErrTotalOverflow},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := NewOrder(c.items)
			if !errors.Is(err, c.want) {
				t.Fatalf("got %v, want %v", err, c.want)
			}
		})
	}
}
func TestOrderOwnsItsItems(t *testing.T) {
	items := []Item{{ProductID: 42, Quantity: 2, Price: 15000}, {ProductID: 17, Quantity: 1, Price: 9900}}
	order, err := NewOrder(items)
	if err != nil {
		t.Fatal(err)
	}
	items[0].Price = 1
	exposed := order.Items()
	exposed[0].Quantity = 100
	if order.Total() != 39900 || order.Items()[0].Quantity != 2 || order.Items()[0].Price != 15000 {
		t.Fatal("order was mutated externally")
	}
}
