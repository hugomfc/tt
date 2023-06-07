package storage

import (
	"context"
	"errors"

	"github.com/hugomfc/tt/internal/domain"
)

var (
	ErrCounterNotFound = errors.New("counter not found")
)

type Storage interface {
	AddRule(ctx context.Context, rule *domain.Rule) error
	UpdateRule(ctx context.Context, rule *domain.Rule) error
	DeleteRule(ctx context.Context, key string) error
	//LoadRules(ctx context.Context) ([]*domain.Rule, error)
	GetRules(ctx context.Context) ([]*domain.Rule, error)
	GetCounter(ctx context.Context, slotSize int, rule *domain.Rule, groupBy string) (*domain.Counter, error)
	IncrementCounterBy(ctx context.Context, rule *domain.Rule, groupBy string, slot int, delta int) (int, error)
}
