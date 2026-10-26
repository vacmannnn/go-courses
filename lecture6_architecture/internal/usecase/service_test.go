package usecase

import (
	"context"
	"errors"
	"github.com/vacmannnn/go-courses/lecture6_architecture/internal/domain"
	"github.com/vacmannnn/go-courses/lecture6_architecture/internal/dto"
	"testing"
)

type catalogStub struct {
	err error
	ctx context.Context
}

func (c *catalogStub) Price(ctx context.Context, id int64) (int64, error) {
	c.ctx = ctx
	return 15000, c.err
}

type ordersSpy struct {
	calls int
	got   domain.Order
	err   error
	ctx   context.Context
}

func (r *ordersSpy) Save(ctx context.Context, o domain.Order) (int64, error) {
	r.calls++
	r.got = o
	r.ctx = ctx
	return 7, r.err
}
func TestCreateOrderOrchestration(t *testing.T) {
	c := &catalogStub{}
	r := &ordersSpy{}
	s := New(c, r)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	output, err := s.CreateOrder(ctx, dto.CreateOrderInput{Items: []dto.OrderItemInput{{ProductID: 42, Quantity: 2}}})
	if err != nil || output.OrderID != 7 || r.calls != 1 || r.got.Total() != 30000 {
		t.Fatalf("output=%+v err=%v saved=%d", output, err, r.calls)
	}
	if c.ctx != ctx || r.ctx != ctx {
		t.Fatal("request context was not propagated")
	}
}
func TestDoesNotSaveFailedOrder(t *testing.T) {
	unavailable := errors.New("catalog unavailable")
	for _, tc := range []struct {
		name             string
		input            dto.CreateOrderInput
		catalogErr, want error
	}{
		{"empty", dto.CreateOrderInput{}, nil, domain.ErrEmptyOrder},
		{"quantity", dto.CreateOrderInput{Items: []dto.OrderItemInput{{ProductID: 42, Quantity: 0}}}, nil, domain.ErrInvalidQuantity},
		{"catalog failure", dto.CreateOrderInput{Items: []dto.OrderItemInput{{ProductID: 42, Quantity: 2}}}, unavailable, unavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &ordersSpy{}
			_, err := New(&catalogStub{err: tc.catalogErr}, r).CreateOrder(context.Background(), tc.input)
			if !errors.Is(err, tc.want) || r.calls != 0 {
				t.Fatalf("err=%v saves=%d", err, r.calls)
			}
		})
	}
}
func TestSaveFailureIsReturned(t *testing.T) {
	failure := errors.New("storage unavailable")
	_, err := New(&catalogStub{}, &ordersSpy{err: failure}).CreateOrder(context.Background(), dto.CreateOrderInput{Items: []dto.OrderItemInput{{ProductID: 42, Quantity: 1}}})
	if !errors.Is(err, failure) {
		t.Fatalf("got %v", err)
	}
}
