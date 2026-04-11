package invoice

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	dbsqlc "rewrite/internal/shared/db/sqlc"
	sharederrors "rewrite/internal/shared/errors"
	"rewrite/internal/shared/utils"
)

type Service struct {
	repo    *Repository
	queries *dbsqlc.Queries
}

func NewService(repo *Repository, db *sql.DB) *Service {
	return &Service{repo: repo, queries: dbsqlc.New(db)}
}

const createScope = "invoices.create"

type CreateInput struct {
	ID            string
	OrderID       string
	InvoiceNumber string
	Status        string
	Metadata      json.RawMessage
}

func (s *Service) Get(ctx context.Context, tenantID, regionID, id string) (Invoice, error) {
	inv, err := s.repo.GetByID(ctx, tenantID, regionID, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Invoice{}, sharederrors.NotFound("invoice not found")
		}
		return Invoice{}, sharederrors.Internal("failed to load invoice")
	}
	return inv, nil
}

func (s *Service) ListByOrder(ctx context.Context, tenantID, regionID, orderID string) ([]Invoice, error) {
	if strings.TrimSpace(orderID) == "" {
		return nil, sharederrors.BadRequest("order_id is required")
	}
	items, err := s.repo.ListByOrder(ctx, tenantID, regionID, orderID)
	if err != nil {
		return nil, sharederrors.Internal("failed to list invoices")
	}
	return items, nil
}

func (s *Service) Create(ctx context.Context, tenantID, regionID string, in CreateInput, idempotencyKey string) (Invoice, error) {
	key := strings.TrimSpace(idempotencyKey)
	if key == "" {
		return Invoice{}, sharederrors.BadRequest("Idempotency-Key is required")
	}
	orderID := strings.TrimSpace(in.OrderID)
	if orderID == "" {
		return Invoice{}, sharederrors.BadRequest("order_id is required")
	}
	st := strings.TrimSpace(strings.ToLower(in.Status))
	if st != "draft" && st != "issued" && st != "void" {
		return Invoice{}, sharederrors.BadRequest("status must be draft, issued, or void")
	}

	tx, err := s.repo.DB.BeginTx(ctx, nil)
	if err != nil {
		return Invoice{}, sharederrors.Internal("failed to begin transaction")
	}
	defer func() { _ = tx.Rollback() }()

	lockKey := tenantID + "\x00" + createScope + "\x00" + key
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1::text))`, lockKey); err != nil {
		return Invoice{}, sharederrors.Internal("failed to lock invoice create")
	}
	qtx := s.queries.WithTx(tx)
	resourceID, idemErr := qtx.GetIdempotencyResource(ctx, dbsqlc.GetIdempotencyResourceParams{
		TenantID:       tenantID,
		Scope:          createScope,
		IdempotencyKey: key,
	})
	if idemErr == nil && resourceID != "" {
		row := tx.QueryRowContext(ctx, `
SELECT id, tenant_id, region_id, order_id, invoice_number, status, total_cents, currency, metadata, created_at, updated_at
FROM order_invoices WHERE id = $1 AND tenant_id = $2 AND region_id = $3
`, resourceID, tenantID, regionID)
		inv, scanErr := scanInvoice(row)
		if errors.Is(scanErr, sql.ErrNoRows) {
			return Invoice{}, sharederrors.Internal("invoice idempotency orphan")
		}
		if scanErr != nil {
			return Invoice{}, sharederrors.Internal("failed to load invoice")
		}
		if err := tx.Commit(); err != nil {
			return Invoice{}, sharederrors.Internal("failed to commit")
		}
		return inv, nil
	}
	if idemErr != nil && !errors.Is(idemErr, sql.ErrNoRows) {
		return Invoice{}, sharederrors.Internal("idempotency lookup failed")
	}

	total, currency, oerr := s.repo.LockOrderTotals(ctx, tx, tenantID, regionID, orderID)
	if errors.Is(oerr, sql.ErrNoRows) {
		return Invoice{}, sharederrors.NotFound(ErrOrderNotFound.Error())
	}
	if oerr != nil {
		return Invoice{}, sharederrors.Internal("failed to load order")
	}

	id := strings.TrimSpace(in.ID)
	if id == "" {
		id = utils.NewID("inv")
	}
	num := strings.TrimSpace(in.InvoiceNumber)
	if num == "" {
		num = "INV-" + id
	}

	inv := Invoice{
		ID: id, TenantID: tenantID, RegionID: regionID, OrderID: orderID,
		InvoiceNumber: num, Status: st, TotalCents: total, Currency: strings.ToUpper(strings.TrimSpace(currency)),
		Metadata: in.Metadata,
	}
	saved, insErr := s.repo.InsertInTx(ctx, tx, inv)
	if insErr != nil {
		return Invoice{}, sharederrors.Internal("failed to create invoice")
	}
	if err := qtx.SaveIdempotencyResource(ctx, dbsqlc.SaveIdempotencyResourceParams{
		TenantID:       tenantID,
		Scope:          createScope,
		IdempotencyKey: key,
		ResourceID:     saved.ID,
	}); err != nil {
		return Invoice{}, sharederrors.Internal("failed to save idempotency")
	}
	if err := tx.Commit(); err != nil {
		return Invoice{}, sharederrors.Internal("failed to commit")
	}
	return saved, nil
}
