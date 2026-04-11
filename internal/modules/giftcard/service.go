package giftcard

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/lib/pq"
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

const createScope = "gift_cards.create"

func (s *Service) List(ctx context.Context, tenantID, regionID string) ([]Card, error) {
	return s.repo.List(ctx, tenantID, regionID)
}

func (s *Service) Create(ctx context.Context, tenantID, regionID string, in CreateInput, idempotencyKey string) (Card, error) {
	key := strings.TrimSpace(idempotencyKey)
	if key == "" {
		return Card{}, sharederrors.BadRequest("Idempotency-Key is required")
	}
	code := strings.ToUpper(strings.TrimSpace(in.Code))
	if code == "" || in.BalanceCents < 0 || len(strings.TrimSpace(in.Currency)) != 3 {
		return Card{}, sharederrors.BadRequest("invalid gift card payload")
	}
	cur := strings.ToUpper(strings.TrimSpace(in.Currency))
	id := strings.TrimSpace(in.ID)
	if id == "" {
		id = utils.NewID("gfc")
	}

	tx, err := s.repo.DB.BeginTx(ctx, nil)
	if err != nil {
		return Card{}, sharederrors.Internal("failed to begin transaction")
	}
	defer func() { _ = tx.Rollback() }()

	lockKey := tenantID + "\x00" + createScope + "\x00" + key
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1::text))`, lockKey); err != nil {
		return Card{}, sharederrors.Internal("failed to lock gift card create")
	}
	qtx := s.queries.WithTx(tx)
	resourceID, idemErr := qtx.GetIdempotencyResource(ctx, dbsqlc.GetIdempotencyResourceParams{
		TenantID:       tenantID,
		Scope:          createScope,
		IdempotencyKey: key,
	})
	if idemErr == nil && resourceID != "" {
		row := tx.QueryRowContext(ctx, `
SELECT id, tenant_id, region_id, code, balance_cents, currency, is_active, expires_at, created_at, updated_at
FROM gift_cards WHERE id = $1 AND tenant_id = $2 AND region_id = $3
`, resourceID, tenantID, regionID)
		c, scanErr := scanCard(row)
		if errors.Is(scanErr, sql.ErrNoRows) {
			return Card{}, sharederrors.Internal("gift card idempotency orphan")
		}
		if scanErr != nil {
			return Card{}, sharederrors.Internal("failed to load gift card")
		}
		if err := tx.Commit(); err != nil {
			return Card{}, sharederrors.Internal("failed to commit")
		}
		return c, nil
	}
	if idemErr != nil && !errors.Is(idemErr, sql.ErrNoRows) {
		return Card{}, sharederrors.Internal("idempotency lookup failed")
	}

	expStr := strings.TrimSpace(in.ExpiresAtRFC3339)
	c := Card{
		ID: id, TenantID: tenantID, RegionID: regionID, Code: code,
		BalanceCents: in.BalanceCents, Currency: cur, IsActive: in.IsActive, ExpiresAt: expStr,
	}
	saved, insErr := s.repo.InsertTx(ctx, tx, c)
	if insErr != nil {
		var pqErr *pq.Error
		if errors.As(insErr, &pqErr) && pqErr.Code == "23505" {
			return Card{}, sharederrors.Conflict(ErrDuplicateCode.Error())
		}
		return Card{}, sharederrors.Internal("failed to create gift card")
	}
	if err := qtx.SaveIdempotencyResource(ctx, dbsqlc.SaveIdempotencyResourceParams{
		TenantID:       tenantID,
		Scope:          createScope,
		IdempotencyKey: key,
		ResourceID:     saved.ID,
	}); err != nil {
		return Card{}, sharederrors.Internal("failed to save idempotency")
	}
	if err := tx.Commit(); err != nil {
		return Card{}, sharederrors.Internal("failed to commit")
	}
	return saved, nil
}
