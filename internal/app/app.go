package app

import (
	"context"
	"errors"
	"net/http"
	"time"

	"rewrite/internal/config"
	"rewrite/internal/modules/brands"
	"rewrite/internal/modules/catalog"
	"rewrite/internal/modules/customers"
	"rewrite/internal/modules/identity"
	"rewrite/internal/modules/inventory"
	"rewrite/internal/modules/localization"
	"rewrite/internal/modules/metadata"
	"rewrite/internal/modules/orders"
	"rewrite/internal/modules/payments"
	"rewrite/internal/modules/pricing"
	"rewrite/internal/modules/promotions"
	"rewrite/internal/modules/regions"
	"rewrite/internal/modules/search"
	"rewrite/internal/modules/shipping"
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

	conn, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	if !db.IsReady(conn) {
		return nil, errors.New("database connection is required")
	}

	outbox := events.NewOutboxStore(conn)
	bus := events.NewBusWithOutbox(outbox)
	webhooks := events.NewWebhookDispatcher(time.Duration(cfg.WebhookTimeoutMS) * time.Millisecond)
	webhooks.Attach(bus)
	events.NewWorker(outbox, webhooks).Start(ctx)

	s := server.New(
		cfg.Port,
		[]func(http.Handler) http.Handler{
			middleware.AccessLog(),
			middleware.Timeout(time.Duration(cfg.HTTPTimeoutMS) * time.Millisecond),
			middleware.BodyLimit(cfg.HTTPMaxBodyBytes),
			middleware.TenantRegion(cfg.DefaultTenantID, cfg.DefaultRegionID),
		},
		catalog.NewHandler(catalog.NewService(catalog.NewRepository(conn), bus)),
		orders.NewHandler(orders.NewService(orders.NewRepository(conn), bus)),
		customers.NewHandler(customers.NewService(customers.NewRepository(conn))),
		inventory.NewHandler(inventory.NewService(inventory.NewRepository(conn), bus)),
		pricing.NewHandler(pricing.NewService(pricing.NewRepository(conn))),
		promotions.NewHandler(promotions.NewService(promotions.NewRepository(conn))),
		regions.NewHandler(regions.NewService(regions.NewRepository(conn))),
		brands.NewHandler(brands.NewService(brands.NewRepository(conn))),
		payments.NewHandler(payments.NewService(payments.NewRepository(conn))),
		shipping.NewHandler(shipping.NewService(shipping.NewRepository(conn))),
		identity.NewHandler(identity.NewService(identity.NewRepository(conn))),
		localization.NewHandler(localization.NewService(localization.NewRepository(conn))),
		metadata.NewHandler(metadata.NewService(metadata.NewRepository(conn))),
		search.NewHandler(search.NewService(search.NewRepository(conn))),
	)
	return &App{srv: s}, nil
}

func (a *App) Run() error {
	return a.srv.Run()
}
