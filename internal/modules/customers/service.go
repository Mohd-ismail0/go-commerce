package customers

import "context"

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Save(_ context.Context, customer Customer) Customer {
	return s.repo.Save(customer)
}

func (s *Service) List(_ context.Context, tenantID string) []Customer {
	return s.repo.List(tenantID)
}
