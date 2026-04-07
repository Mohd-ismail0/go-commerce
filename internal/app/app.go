package app

import (
	"context"
	"net/http"
	"time"

	"rewrite/internal/config"
	"rewrite/internal/modules/catalog"
	"rewrite/internal/modules/customers"
	"rewrite/internal/modules/inventory"
	"rewrite/internal/modules/orders"
	"rewrite/internal/modules/pricing"
	"rewrite/internal/modules/promotions"
	"rewrite/internal/modules/regions"
	"rewrite/internal/modules/brands"
	"rewrite/internal/server"
	"rewrite/internal/shared/db"
	"rewrite/internal/shared/events"
	"rewrite/internal/shared/middleware"
)

type App struct {
	srv *server.Server
}

func New() (*App, error) {
	cfg := config.Load()
	ctx := context.Background()

	_, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	bus := events.NewBus()
	webhooks := events.NewWebhookDispatcher(time.Duration(cfg.WebhookTimeoutMS) * time.Millisecond)
	webhooks.Attach(bus)

	s := server.New(
		cfg.Port,
		[]func(http.Handler) http.Handler{
			middleware.TenantRegion(cfg.DefaultTenantID, cfg.DefaultRegionID),
		},
		catalog.NewHandler(catalog.NewService(catalog.NewRepository(), bus)),
		orders.NewHandler(orders.NewService(orders.NewRepository(), bus)),
		customers.NewHandler(customers.NewService(customers.NewRepository())),
		inventory.NewHandler(inventory.NewService(inventory.NewRepository(), bus)),
		pricing.NewHandler(pricing.NewService(pricing.NewRepository())),
		promotions.NewHandler(promotions.NewService(promotions.NewRepository())),
		regions.NewHandler(regions.NewService(regions.NewRepository())),
		brands.NewHandler(brands.NewService(brands.NewRepository())),
	)
	return &App{srv: s}, nil
}

func (a *App) Run() error {
	return a.srv.Run()
}
