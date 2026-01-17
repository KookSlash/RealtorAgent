package listings

import (
	"context"
	"time"
)

type Listing struct {
	PropertyKey string
	Address     string
	PostalCode  string
	Price       *float64
	Beds        *int
	Baths       *float64
	Sqft        *int
	URL         *string
	ScrapedAt   time.Time
}

type ListingsRepository interface {
	Count(ctx context.Context) (int64, error)
	List(ctx context.Context, limit, offset int) ([]Listing, error)
}

type Service struct {
	repo ListingsRepository
}

func NewService(repo ListingsRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}

func (s *Service) List(ctx context.Context, limit, offset int) ([]Listing, error) {
	return s.repo.List(ctx, limit, offset)
}
