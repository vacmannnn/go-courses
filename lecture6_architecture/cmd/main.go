package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/vacmannnn/go-courses/lecture6_architecture/internal/adapter"
	"github.com/vacmannnn/go-courses/lecture6_architecture/internal/config"
	"github.com/vacmannnn/go-courses/lecture6_architecture/internal/controller"
	"github.com/vacmannnn/go-courses/lecture6_architecture/internal/dto"
	"github.com/vacmannnn/go-courses/lecture6_architecture/internal/usecase"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		slog.Error("application stopped", "error", err)
		os.Exit(1)
	}
}
func run() error {
	mode := flag.String("mode", "http", "http or cli")
	product := flag.Int64("product", 42, "product ID for CLI")
	quantity := flag.Int("quantity", 2, "quantity for CLI")
	flag.Parse()
	cfg := config.Load()
	logger := slog.Default()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	var catalog usecase.Catalog
	var orders usecase.OrderRepository
	if cfg.DatabaseURL == "" {
		catalog = adapter.NewMemoryCatalog(map[int64]int64{42: 15000, 17: 9900})
		orders = adapter.NewMemoryOrders()
	} else {
		db, err := sql.Open("pgx", cfg.DatabaseURL)
		if err != nil {
			return err
		}
		defer db.Close()
		pingCtx, stop := context.WithTimeout(ctx, 3*time.Second)
		err = db.PingContext(pingCtx)
		stop()
		if err != nil {
			return err
		}
		catalog = adapter.NewPostgresCatalog(db)
		orders = adapter.NewPostgresOrders(db)
	}
	service := usecase.New(catalog, orders)
	switch *mode {
	case "cli":
		output, err := service.CreateOrder(ctx, dto.CreateOrderInput{Items: []dto.OrderItemInput{{ProductID: *product, Quantity: *quantity}}})
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(output)
	case "http":
		handler := controller.New(service, logger)
		server := &http.Server{Addr: cfg.Address, Handler: handler.Routes(), ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second}
		stopped := make(chan error, 1)
		go func() { stopped <- server.ListenAndServe() }()
		fmt.Println("listening on http://" + cfg.Address)
		select {
		case err := <-stopped:
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			return err
		case <-ctx.Done():
			shutdownCtx, stop := context.WithTimeout(context.Background(), 3*time.Second)
			defer stop()
			return server.Shutdown(shutdownCtx)
		}
	default:
		return fmt.Errorf("unknown mode %q", *mode)
	}
}
