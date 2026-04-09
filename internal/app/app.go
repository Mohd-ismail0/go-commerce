package app

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"rewrite/internal/config"
	"rewrite/internal/modules/apps"
	"rewrite/internal/modules/brands"
	"rewrite/internal/modules/catalog"
	"rewrite/internal/modules/channels"
	"rewrite/internal/modules/checkout"
	"rewrite/internal/modules/customers"
	"rewrite/internal/modules/fulfillments"
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
	"rewrite/internal/modules/webhooks"
	"rewrite/internal/server"
	"rewrite/internal/shared/db"
	"rewrite/internal/shared/events"
	"rewrite/internal/shared/middleware"
)

type App struct {
	srv    *server.Server
	dbConn *sql.DB
	cancel context.CancelFunc
}

func New(ctx context.Context) (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	appCtx, cancel := context.WithCancel(ctx)

	conn, err := db.Connect(appCtx, cfg.DatabaseURL)
	if err != nil {
		cancel()
		return nil, err
	}
	if !db.IsReady(conn) {
		cancel()
		return nil, errors.New("database connection is required")
	}

	outbox := events.NewOutboxStore(conn)
	bus := events.NewBusWithOutbox(outbox)
	webhookDispatcher := events.NewWebhookDispatcher(time.Duration(cfg.WebhookTimeoutMS) * time.Millisecond)
	webhookDispatcher.Attach(bus)
	events.NewWorker(outbox, webhookDispatcher).Start(appCtx)
	promotionsSvc := promotions.NewService(promotions.NewRepository(conn))
	pricingSvc := pricing.NewService(pricing.NewRepository(conn), promotionsSvc)
	paymentSvc := payments.NewService(payments.NewRepository(conn))
	orderSvc := orders.NewService(orders.NewRepository(conn), bus, pricingSvc)
	payments.StartReconciliationWorker(
		appCtx,
		paymentSvc,
		bus,
		cfg.DefaultTenantID,
		cfg.DefaultRegionID,
		time.Duration(cfg.PaymentReconcileIntervalSeconds)*time.Second,
	)
	jwtKeys := parseJWTKeys(cfg.AuthJWTSecret, cfg.AuthJWTKeyset)

	s := server.New(
		cfg.Port,
		[]func(http.Handler) http.Handler{
			middleware.AccessLog(),
			middleware.APIToken(cfg.APIAuthToken),
			middleware.TenantRegion(cfg.DefaultTenantID, cfg.DefaultRegionID),
			middleware.PolicyAuthorization(conn, []middleware.PolicyRule{
				{Prefix: "/payments", PermissionCode: "payments.manage"},
				{Prefix: "/shipping", PermissionCode: "shipping.manage"},
				{Prefix: "/identity/users", PermissionCode: "identity.users.manage"},
				{Prefix: "/metadata", PermissionCode: "metadata.manage"},
				{Prefix: "/apps/webhook-subscriptions", PermissionCode: "webhook.manage"},
				{Prefix: "/channels", PermissionCode: "channel.manage"},
			}, middleware.PolicyOptions{
				UserJWTSecret:         cfg.AuthJWTSecret,
				UserJWTKeys:           jwtKeys,
				AllowLegacyRoleBypass: cfg.AllowLegacyRoleBypass,
			}),
			middleware.Timeout(time.Duration(cfg.HTTPTimeoutMS) * time.Millisecond),
			middleware.BodyLimit(cfg.HTTPMaxBodyBytes),
		},
		func(ctx context.Context) error {
			return conn.PingContext(ctx)
		},
		catalog.NewHandler(catalog.NewService(catalog.NewRepository(conn), bus)),
		checkout.NewHandler(checkout.NewService(checkout.NewRepository(conn), bus, pricingSvc)),
		orders.NewHandler(orderSvc),
		fulfillments.NewHandler(fulfillments.NewService(fulfillments.NewRepository(conn), orderSvc)),
		customers.NewHandler(customers.NewService(customers.NewRepository(conn))),
		inventory.NewHandler(inventory.NewService(inventory.NewRepository(conn), bus)),
		pricing.NewHandler(pricingSvc),
		promotions.NewHandler(promotionsSvc),
		regions.NewHandler(regions.NewService(regions.NewRepository(conn))),
		channels.NewHandler(channels.NewService(channels.NewRepository(conn))),
		brands.NewHandler(brands.NewService(brands.NewRepository(conn))),
		payments.NewHandler(paymentSvc, cfg.WebhookPaymentSecret, cfg.AppEnv),
		shipping.NewHandler(shipping.NewService(shipping.NewRepository(conn))),
		apps.NewHandler(apps.NewService(apps.NewRepository(conn))),
		webhooks.NewHandler(webhooks.NewService(webhooks.NewRepository(conn))),
		identity.NewHandler(identity.NewService(identity.NewRepository(conn), cfg.AuthJWTSecret, cfg.AuthJWTKeyset, cfg.AuthJWTTTLMinutes, cfg.AuthRefreshTTLMinutes)),
		localization.NewHandler(localization.NewService(localization.NewRepository(conn))),
		metadata.NewHandler(metadata.NewService(metadata.NewRepository(conn))),
		search.NewHandler(search.NewService(search.NewRepository(conn))),
	)
	return &App{
		srv:    s,
		dbConn: conn,
		cancel: cancel,
	}, nil
}

func parseJWTKeys(legacySecret, keyset string) []middleware.JWTKey {
	out := []middleware.JWTKey{}
	for _, pair := range strings.Split(strings.TrimSpace(keyset), ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			continue
		}
		kid := strings.TrimSpace(parts[0])
		secret := strings.TrimSpace(parts[1])
		if kid == "" || secret == "" {
			continue
		}
		out = append(out, middleware.JWTKey{ID: kid, Secret: secret})
	}
	if len(out) == 0 && strings.TrimSpace(legacySecret) != "" {
		out = append(out, middleware.JWTKey{ID: "legacy", Secret: strings.TrimSpace(legacySecret)})
	}
	return out
}

func (a *App) Run() error {
	return a.srv.Run()
}

func (a *App) Shutdown(ctx context.Context) error {
	a.cancel()
	if err := a.srv.Shutdown(ctx); err != nil {
		return err
	}
	return a.dbConn.Close()
}
