package channels

import (
	"context"
	"database/sql"
	"strings"
	"time"

	sharederrors "rewrite/internal/shared/errors"
	"rewrite/internal/shared/utils"
)

type repo interface {
	SlugTaken(ctx context.Context, tenantID, regionID, slug, excludeID string) (bool, error)
	Save(ctx context.Context, ch Channel) (Channel, error)
	List(ctx context.Context, tenantID, regionID string) ([]Channel, error)
	ChannelExists(ctx context.Context, tenantID, regionID, channelID string) (bool, error)
	ProductExists(ctx context.Context, tenantID, regionID, productID string) (bool, error)
	GetProductListingByKeys(ctx context.Context, tenantID, regionID, channelID, productID string) (ProductChannelListing, bool, error)
	ListProductListingsByChannel(ctx context.Context, tenantID, regionID, channelID string) ([]ProductChannelListing, error)
	SaveProductListing(ctx context.Context, row ProductChannelListing, publishedAt sql.NullTime) (ProductChannelListing, error)
}

type Service struct {
	repo repo
}

func NewService(r repo) *Service {
	return &Service{repo: r}
}

func (s *Service) List(ctx context.Context, tenantID, regionID string) ([]Channel, error) {
	return s.repo.List(ctx, tenantID, regionID)
}

func (s *Service) Save(ctx context.Context, ch Channel) (Channel, error) {
	if strings.TrimSpace(ch.Slug) == "" || strings.TrimSpace(ch.Name) == "" {
		return Channel{}, sharederrors.BadRequest("slug and name are required")
	}
	ch.Slug = strings.TrimSpace(ch.Slug)
	ch.Name = strings.TrimSpace(ch.Name)
	ch.DefaultCurrency = strings.ToUpper(strings.TrimSpace(ch.DefaultCurrency))
	ch.DefaultCountry = strings.ToUpper(strings.TrimSpace(ch.DefaultCountry))
	if len(ch.DefaultCurrency) != 3 {
		return Channel{}, sharederrors.BadRequest("default_currency must be ISO 4217 (3 letters)")
	}
	if len(ch.DefaultCountry) != 2 {
		return Channel{}, sharederrors.BadRequest("default_country must be a 2-letter ISO code")
	}
	if ch.ID == "" {
		ch.ID = utils.NewID("chn")
	}
	taken, err := s.repo.SlugTaken(ctx, ch.TenantID, ch.RegionID, ch.Slug, ch.ID)
	if err != nil {
		return Channel{}, sharederrors.Internal("failed to validate channel slug")
	}
	if taken {
		return Channel{}, sharederrors.Conflict("channel slug already exists in tenant/region")
	}
	saved, err := s.repo.Save(ctx, ch)
	if err != nil {
		return Channel{}, sharederrors.Internal("failed to save channel")
	}
	return saved, nil
}

func (s *Service) ListProductListings(ctx context.Context, tenantID, regionID, channelID string) ([]ProductChannelListing, error) {
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return nil, sharederrors.BadRequest("channel id is required")
	}
	ok, err := s.repo.ChannelExists(ctx, tenantID, regionID, channelID)
	if err != nil {
		return nil, sharederrors.Internal("failed to validate channel")
	}
	if !ok {
		return nil, sharederrors.NotFound("channel not found")
	}
	return s.repo.ListProductListingsByChannel(ctx, tenantID, regionID, channelID)
}

// ProductListingInput is the mutable part of a listing (used for POST/PATCH).
type ProductListingInput struct {
	ID                string
	ProductID         string
	PublishedAt       *string
	IsPublished       *bool
	VisibleInListings *bool
}

func (s *Service) UpsertProductListing(ctx context.Context, tenantID, regionID, channelID string, in ProductListingInput, patch bool) (ProductChannelListing, error) {
	channelID = strings.TrimSpace(channelID)
	in.ProductID = strings.TrimSpace(in.ProductID)
	if channelID == "" || in.ProductID == "" {
		return ProductChannelListing{}, sharederrors.BadRequest("channel id and product_id are required")
	}

	chOK, err := s.repo.ChannelExists(ctx, tenantID, regionID, channelID)
	if err != nil {
		return ProductChannelListing{}, sharederrors.Internal("failed to validate channel")
	}
	if !chOK {
		return ProductChannelListing{}, sharederrors.NotFound("channel not found")
	}
	pOK, err := s.repo.ProductExists(ctx, tenantID, regionID, in.ProductID)
	if err != nil {
		return ProductChannelListing{}, sharederrors.Internal("failed to validate product")
	}
	if !pOK {
		return ProductChannelListing{}, sharederrors.NotFound("product not found")
	}

	row := ProductChannelListing{
		TenantID:  tenantID,
		RegionID:  regionID,
		ChannelID: channelID,
		ProductID: in.ProductID,
	}

	if patch {
		ex, ok, err := s.repo.GetProductListingByKeys(ctx, tenantID, regionID, channelID, in.ProductID)
		if err != nil {
			return ProductChannelListing{}, sharederrors.Internal("failed to load listing")
		}
		if !ok {
			return ProductChannelListing{}, sharederrors.NotFound("product channel listing not found")
		}
		row = ex
		if in.IsPublished != nil {
			row.IsPublished = *in.IsPublished
		}
		if in.VisibleInListings != nil {
			row.VisibleInListings = *in.VisibleInListings
		}
	} else {
		row.IsPublished = false
		row.VisibleInListings = true
		if in.IsPublished != nil {
			row.IsPublished = *in.IsPublished
		}
		if in.VisibleInListings != nil {
			row.VisibleInListings = *in.VisibleInListings
		}
		if strings.TrimSpace(in.ID) != "" {
			row.ID = strings.TrimSpace(in.ID)
		}
		if row.ID == "" {
			row.ID = utils.NewID("pcl")
		}
	}

	pub, err := listingPublishedAt(in.PublishedAt, row.PublishedAt, patch)
	if err != nil {
		return ProductChannelListing{}, err
	}

	saved, err := s.repo.SaveProductListing(ctx, row, pub)
	if err != nil {
		return ProductChannelListing{}, sharederrors.Internal("failed to save product channel listing")
	}
	return saved, nil
}

func listingPublishedAt(in *string, existingRFC3339 string, patch bool) (sql.NullTime, error) {
	if patch {
		if in == nil {
			return publishedAtFromExisting(existingRFC3339)
		}
		s := strings.TrimSpace(*in)
		if s == "" {
			return sql.NullTime{}, nil
		}
		return parsePublishedAt(s)
	}
	if in == nil {
		return sql.NullTime{}, nil
	}
	s := strings.TrimSpace(*in)
	if s == "" {
		return sql.NullTime{}, nil
	}
	return parsePublishedAt(s)
}

func publishedAtFromExisting(existingRFC3339 string) (sql.NullTime, error) {
	s := strings.TrimSpace(existingRFC3339)
	if s == "" {
		return sql.NullTime{}, nil
	}
	return parsePublishedAt(s)
}

func parsePublishedAt(s string) (sql.NullTime, error) {
	t, perr := time.Parse(time.RFC3339Nano, s)
	if perr != nil {
		t, perr = time.Parse(time.RFC3339, s)
	}
	if perr != nil {
		return sql.NullTime{}, sharederrors.BadRequest("published_at must be RFC3339")
	}
	return sql.NullTime{Time: t.UTC(), Valid: true}, nil
}
