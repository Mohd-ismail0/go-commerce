package search

import "context"

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Save(_ context.Context, item Document) Document {
	return s.repo.Save(item)
}

func (s *Service) Query(_ context.Context, tenantID, regionID, entityType, query string, limit int) []SearchHit {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.repo.Query(tenantID, regionID, entityType, query, limit)
}
