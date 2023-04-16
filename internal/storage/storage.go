package storage

import (
	"context"
	"github.com/hugomfc/tt/internal/domain"
)

type Storage interface {
  AddRule(ctx context.Context, rule *domain.Rule) error
  UpdateRule(ctx context.Context, rule *domain.Rule) error
  DeleteRule(ctx context.Context, key string) error 
	//LoadRules(ctx context.Context) ([]*domain.Rule, error)
  GetRules(ctx context.Context) ([]*domain.Rule, error)
  GetCounter(ctx context.Context, rule *domain.Rule, groupBy string) (*domain.Slots, error) 
  IncrementCounterBy(ctx context.Context, rule *domain.Rule, groupBy string, slot int, delta int) (int, error) 
}

