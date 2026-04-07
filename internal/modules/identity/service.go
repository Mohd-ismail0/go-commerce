package identity

import "context"

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Save(_ context.Context, item User) User {
	return s.repo.Save(item)
}

func (s *Service) List(_ context.Context, tenantID string) []User {
	return s.repo.List(tenantID)
}
