package customers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	shareddb "rewrite/internal/shared/db"
	dbsqlc "rewrite/internal/shared/db/sqlc"
)

const customerSaveIdempotencyScope = "customers.save"

type Repository struct {
	db      *sql.DB
	queries *dbsqlc.Queries
}

var ErrCustomerNotFound = errors.New("customer not found")
var ErrAddressNotFound = errors.New("address not found")
var ErrIdempotencyKeyRequired = errors.New("customer idempotency key is required")
var ErrCustomerIdempotencyOrphan = errors.New("idempotency record references missing customer")
var ErrCustomerEmailTaken = errors.New("customer email already exists in tenant/region")

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db, queries: dbsqlc.New(db)}
}

func (r *Repository) Save(ctx context.Context, customer Customer, idempotencyKey string) (Customer, error) {
	key := strings.TrimSpace(idempotencyKey)
	if key == "" {
		return Customer{}, ErrIdempotencyKeyRequired
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Customer{}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	lockKey := customer.TenantID + "\x00" + customerSaveIdempotencyScope + "\x00" + key
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1::text))`, lockKey); err != nil {
		return Customer{}, err
	}

	qtx := r.queries.WithTx(tx)
	resourceID, idemErr := qtx.GetIdempotencyResource(ctx, dbsqlc.GetIdempotencyResourceParams{
		TenantID:       customer.TenantID,
		Scope:          customerSaveIdempotencyScope,
		IdempotencyKey: key,
	})
	if idemErr == nil && resourceID != "" {
		row := tx.QueryRowContext(ctx, `
SELECT id, tenant_id, region_id, email, name FROM customers
WHERE id = $1 AND tenant_id = $2 AND region_id = $3
`, resourceID, customer.TenantID, customer.RegionID)
		var out Customer
		if err := row.Scan(&out.ID, &out.TenantID, &out.RegionID, &out.Email, &out.Name); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return Customer{}, fmt.Errorf("%w: %q", ErrCustomerIdempotencyOrphan, resourceID)
			}
			return Customer{}, err
		}
		if err := tx.Commit(); err != nil {
			return Customer{}, err
		}
		return out, nil
	}
	if idemErr != nil && !errors.Is(idemErr, sql.ErrNoRows) {
		return Customer{}, idemErr
	}

	taken, err := emailTakenTx(ctx, tx, customer.TenantID, customer.RegionID, customer.Email, customer.ID)
	if err != nil {
		return Customer{}, err
	}
	if taken {
		return Customer{}, ErrCustomerEmailTaken
	}

	var out Customer
	err = tx.QueryRowContext(ctx, `
INSERT INTO customers (id, tenant_id, region_id, email, name, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
tenant_id = EXCLUDED.tenant_id,
region_id = EXCLUDED.region_id,
email = EXCLUDED.email,
name = EXCLUDED.name,
updated_at = NOW()
RETURNING id, tenant_id, region_id, email, name
`, customer.ID, customer.TenantID, customer.RegionID, customer.Email, customer.Name).Scan(
		&out.ID, &out.TenantID, &out.RegionID, &out.Email, &out.Name,
	)
	if err != nil {
		if shareddb.IsUniqueConstraintViolation(err, "ux_customers_tenant_region_email_ci") {
			return Customer{}, ErrCustomerEmailTaken
		}
		return Customer{}, err
	}

	if err := qtx.SaveIdempotencyResource(ctx, dbsqlc.SaveIdempotencyResourceParams{
		TenantID:       customer.TenantID,
		Scope:          customerSaveIdempotencyScope,
		IdempotencyKey: key,
		ResourceID:     out.ID,
	}); err != nil {
		return Customer{}, err
	}
	if err := tx.Commit(); err != nil {
		return Customer{}, err
	}
	return out, nil
}

func emailTakenTx(ctx context.Context, tx *sql.Tx, tenantID, regionID, email, excludeID string) (bool, error) {
	var exists bool
	err := tx.QueryRowContext(ctx, `
SELECT EXISTS(
	SELECT 1 FROM customers
	WHERE tenant_id = $1
	  AND region_id = $2
	  AND LOWER(email) = LOWER($3)
	  AND ($4::text = '' OR id <> $4)
)
`, tenantID, regionID, email, excludeID).Scan(&exists)
	return exists, err
}

func (r *Repository) List(ctx context.Context, tenantID, regionID string) ([]Customer, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, tenant_id, region_id, email, name FROM customers
WHERE tenant_id = $1 AND region_id = $2 ORDER BY updated_at DESC
`, tenantID, regionID)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	out := []Customer{}
	for rows.Next() {
		var c Customer
		if err := rows.Scan(&c.ID, &c.TenantID, &c.RegionID, &c.Email, &c.Name); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repository) EmailTaken(ctx context.Context, tenantID, regionID, email, excludeID string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `
SELECT EXISTS(
	SELECT 1 FROM customers
	WHERE tenant_id = $1
	  AND region_id = $2
	  AND LOWER(email) = LOWER($3)
	  AND ($4::text = '' OR id <> $4)
)
`, tenantID, regionID, email, excludeID).Scan(&exists)
	return exists, err
}

func (r *Repository) customerExists(ctx context.Context, tenantID, regionID, customerID string) (bool, error) {
	var ok bool
	err := r.db.QueryRowContext(ctx, `
SELECT EXISTS(
  SELECT 1 FROM customers WHERE id = $1 AND tenant_id = $2 AND region_id = $3
)
`, customerID, tenantID, regionID).Scan(&ok)
	return ok, err
}

func lockCustomerTx(ctx context.Context, tx *sql.Tx, tenantID, regionID, customerID string) error {
	var x int
	err := tx.QueryRowContext(ctx, `
SELECT 1 FROM customers WHERE id = $1 AND tenant_id = $2 AND region_id = $3 FOR UPDATE
`, customerID, tenantID, regionID).Scan(&x)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrCustomerNotFound
	}
	return err
}

func (r *Repository) ListAddresses(ctx context.Context, tenantID, regionID, customerID string) ([]Address, error) {
	ok, err := r.customerExists(ctx, tenantID, regionID, customerID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrCustomerNotFound
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT id, tenant_id, region_id, customer_id, is_default_shipping, is_default_billing,
  first_name, last_name, COALESCE(company,''), street_line_1, COALESCE(street_line_2,''), city, postal_code,
  UPPER(country_code), COALESCE(phone,''), created_at, updated_at
FROM customer_addresses
WHERE tenant_id = $1 AND region_id = $2 AND customer_id = $3
ORDER BY is_default_shipping DESC, is_default_billing DESC, updated_at DESC
`, tenantID, regionID, customerID)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	out := make([]Address, 0)
	for rows.Next() {
		a, scanErr := scanAddressRow(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *Repository) SaveAddress(ctx context.Context, a Address) (Address, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Address{}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := lockCustomerTx(ctx, tx, a.TenantID, a.RegionID, a.CustomerID); err != nil {
		return Address{}, err
	}

	if a.IsDefaultShipping {
		if _, err := tx.ExecContext(ctx, `
UPDATE customer_addresses
SET is_default_shipping = FALSE, updated_at = NOW()
WHERE tenant_id = $1 AND region_id = $2 AND customer_id = $3 AND id <> $4
`, a.TenantID, a.RegionID, a.CustomerID, a.ID); err != nil {
			return Address{}, err
		}
	}
	if a.IsDefaultBilling {
		if _, err := tx.ExecContext(ctx, `
UPDATE customer_addresses
SET is_default_billing = FALSE, updated_at = NOW()
WHERE tenant_id = $1 AND region_id = $2 AND customer_id = $3 AND id <> $4
`, a.TenantID, a.RegionID, a.CustomerID, a.ID); err != nil {
			return Address{}, err
		}
	}

	var dummy int
	exErr := tx.QueryRowContext(ctx, `
SELECT 1 FROM customer_addresses
WHERE id = $1 AND tenant_id = $2 AND region_id = $3 AND customer_id = $4
`, a.ID, a.TenantID, a.RegionID, a.CustomerID).Scan(&dummy)
	isUpdate := exErr == nil
	if exErr != nil && !errors.Is(exErr, sql.ErrNoRows) {
		return Address{}, exErr
	}

	var out Address
	var scanErr error
	if isUpdate {
		out, scanErr = scanAddressRow(tx.QueryRowContext(ctx, `
UPDATE customer_addresses
SET is_default_shipping = $5, is_default_billing = $6,
    first_name = $7, last_name = $8, company = NULLIF($9,''), street_line_1 = $10, street_line_2 = NULLIF($11,''),
    city = $12, postal_code = $13, country_code = UPPER($14), phone = NULLIF($15,''), updated_at = NOW()
WHERE id = $1 AND tenant_id = $2 AND region_id = $3 AND customer_id = $4
RETURNING id, tenant_id, region_id, customer_id, is_default_shipping, is_default_billing,
  first_name, last_name, COALESCE(company,''), street_line_1, COALESCE(street_line_2,''), city, postal_code,
  UPPER(country_code), COALESCE(phone,''), created_at, updated_at
`, a.ID, a.TenantID, a.RegionID, a.CustomerID, a.IsDefaultShipping, a.IsDefaultBilling,
			a.FirstName, a.LastName, a.Company, a.StreetLine1, a.StreetLine2, a.City, a.PostalCode, a.CountryCode, a.Phone))
	} else {
		out, scanErr = scanAddressRow(tx.QueryRowContext(ctx, `
INSERT INTO customer_addresses (
  id, tenant_id, region_id, customer_id, is_default_shipping, is_default_billing,
  first_name, last_name, company, street_line_1, street_line_2, city, postal_code, country_code, phone, created_at, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9,''),$10,NULLIF($11,''),$12,$13,UPPER($14),NULLIF($15,''),NOW(),NOW())
RETURNING id, tenant_id, region_id, customer_id, is_default_shipping, is_default_billing,
  first_name, last_name, COALESCE(company,''), street_line_1, COALESCE(street_line_2,''), city, postal_code,
  UPPER(country_code), COALESCE(phone,''), created_at, updated_at
`, a.ID, a.TenantID, a.RegionID, a.CustomerID, a.IsDefaultShipping, a.IsDefaultBilling,
			a.FirstName, a.LastName, a.Company, a.StreetLine1, a.StreetLine2, a.City, a.PostalCode, a.CountryCode, a.Phone))
	}
	if scanErr != nil {
		if errors.Is(scanErr, sql.ErrNoRows) {
			return Address{}, ErrAddressNotFound
		}
		return Address{}, scanErr
	}
	if err := tx.Commit(); err != nil {
		return Address{}, err
	}
	return out, nil
}

func (r *Repository) DeleteAddress(ctx context.Context, tenantID, regionID, customerID, addressID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := lockCustomerTx(ctx, tx, tenantID, regionID, customerID); err != nil {
		return err
	}

	res, err := tx.ExecContext(ctx, `
DELETE FROM customer_addresses
WHERE id = $1 AND tenant_id = $2 AND region_id = $3 AND customer_id = $4
`, addressID, tenantID, regionID, customerID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrAddressNotFound
	}
	return tx.Commit()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanAddressRow(row scanner) (Address, error) {
	var a Address
	var createdAt, updatedAt time.Time
	err := row.Scan(
		&a.ID, &a.TenantID, &a.RegionID, &a.CustomerID, &a.IsDefaultShipping, &a.IsDefaultBilling,
		&a.FirstName, &a.LastName, &a.Company, &a.StreetLine1, &a.StreetLine2, &a.City, &a.PostalCode,
		&a.CountryCode, &a.Phone, &createdAt, &updatedAt,
	)
	if err != nil {
		return Address{}, err
	}
	a.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	a.UpdatedAt = updatedAt.UTC().Format(time.RFC3339Nano)
	return a, nil
}
