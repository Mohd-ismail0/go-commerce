package pricing

import "context"

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Save(_ context.Context, entry PriceBookEntry) PriceBookEntry {
	return s.repo.Save(entry)
}

func (s *Service) List(_ context.Context, tenantID string) []PriceBookEntry {
	return s.repo.List(tenantID)
}
