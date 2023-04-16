package ratelimiter_test

import (
	"context"
	"testing"
	//"time"
  "fmt"
	"github.com/hugomfc/tt/internal/domain"
	"github.com/hugomfc/tt/internal/ratelimiter"

	//	"github.com/nxadm/tail/watch"
	"github.com/stretchr/testify/assert"
)

type mockStorage struct {
    rules     []*domain.Rule
    counters  map[string]domain.Slots
    addErr    error
    updateErr error
    deleteErr error
    getErr    error
    incrErr   error
}

func (s *mockStorage) AddRule(ctx context.Context, rule *domain.Rule) error {
    s.rules = append(s.rules, rule)
    return s.addErr
}

func (s *mockStorage) UpdateRule(ctx context.Context, rule *domain.Rule) error {
    for i, r := range s.rules {
        if r.ID == rule.ID {
            s.rules[i] = rule
            return s.updateErr
        }
    }
    return fmt.Errorf("rule with ID %s not found", rule.ID)
}

func (s *mockStorage) DeleteRule(ctx context.Context, key string) error {
    for i, rule := range s.rules {
        if rule.ID == key {
            s.rules = append(s.rules[:i], s.rules[i+1:]...)
            return s.deleteErr
        }
    }
    return fmt.Errorf("rule with ID %s not found", key)
}

func (s *mockStorage) GetRules(ctx context.Context) ([]*domain.Rule, error) {
    return s.rules, s.getErr
}

func (s *mockStorage) GetCounter(ctx context.Context, rule *domain.Rule, groupBy string) (*domain.Slots, error) {
    key := fmt.Sprintf("counters:%s:%s", rule.ID, groupBy)
    if slots, ok := s.counters[key]; ok {
        return &slots, s.getErr
    }
    return nil, fmt.Errorf("counter not found for key %s", key)
}

func (s *mockStorage) IncrementCounterBy(ctx context.Context, rule *domain.Rule, groupBy string, slot int, delta int) (int, error) {
  key := fmt.Sprintf("counters:%s:%s", rule.ID, groupBy)
  slots, ok := s.counters[key]
  if !ok {
    slots = make(domain.Slots, rule.WindowSize/1) //TODO: 1 should be slotSize
    s.counters[key] = slots
  }  
   //count, err := slots.IncrementBy(slot, delta)
  slots[slot] += delta
  count := slots[slot]
  return count, s.incrErr
}

func TestRateLimiter_Allow(t *testing.T) {
	storage := &mockStorage{}
	rl := ratelimiter.New(storage, 5, 1)

	requestInfo := &ratelimiter.RequestInfo{
		IP:        "127.0.0.1",
		Method:    "GET",
		Path:      "/",
    //UserAgent: "TestAgent",
		Headers:   map[string]string{"Header": "Value"},
	}

	allowed, ruleId, err := rl.Allow(context.Background(), requestInfo)
	assert.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, "", ruleId)
}

