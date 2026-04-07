package regions

import "context"

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Save(_ context.Context, item Region) Region {
	return s.repo.Save(item)
}

func (s *Service) List(_ context.Context, tenantID string) []Region {
	return s.repo.List(tenantID)
}
