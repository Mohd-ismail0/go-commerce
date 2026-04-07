package brands

import "context"

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Save(_ context.Context, brand Brand) Brand {
	return s.repo.Save(brand)
}

func (s *Service) List(_ context.Context, tenantID string) []Brand {
	return s.repo.List(tenantID)
}
