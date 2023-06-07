package ratelimiter_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hugomfc/tt/internal/domain"
	"github.com/hugomfc/tt/internal/ratelimiter"
	"github.com/stretchr/testify/assert"
)

type mockStorage struct {
	rules    []*domain.Rule
	counters map[string]*domain.Counter
	getErr   error
}

func (s *mockStorage) AddRule(ctx context.Context, rule *domain.Rule) error {
	s.rules = append(s.rules, rule)
	return nil
}

func (s *mockStorage) DeleteRule(ctx context.Context, ruleID string) error {
	for i, rule := range s.rules {
		if rule.ID == ruleID {
			s.rules = append(s.rules[:i], s.rules[i+1:]...)
			break
		}
	}
	return nil
}

func (s *mockStorage) GetRules(ctx context.Context) ([]*domain.Rule, error) {
	return s.rules, nil
}

func (s *mockStorage) IncrementCounterBy(ctx context.Context, rule *domain.Rule, groupBy string, slot int, delta int) (int, error) {
	key := fmt.Sprintf("counters:%s:%s", rule.ID, groupBy)
	if _, ok := s.counters[key]; !ok {
		s.counters[key] = domain.NewCounter(rule, 1)
	}
	s.counters[key].Slots[slot] += delta
	return s.counters[key].Slots[slot], nil
}

func (s *mockStorage) GetCounter(ctx context.Context, slotSize int, rule *domain.Rule, groupBy string) (*domain.Counter, error) {
	key := fmt.Sprintf("counters:%s:%s", rule.ID, groupBy)
	if counter, ok := s.counters[key]; ok {
		return counter, s.getErr
	}
	return nil, fmt.Errorf("counter not found for key %s", key)
}

func (s *mockStorage) UpdateRule(ctx context.Context, rule *domain.Rule) error {
	for i, r := range s.rules {
		if r.ID == rule.ID {
			s.rules[i] = rule
			break
		}
	}
	return nil
}

func TestRateLimiter_Allow(t *testing.T) {
	storage := &mockStorage{}
	rl := ratelimiter.New(storage, 5, 1)

	requestInfo := &ratelimiter.RequestInfo{
		IP:     "127.0.0.1",
		Method: "GET",
		Path:   "/",
		//UserAgent: "TestAgent",
		Headers: map[string]string{"Header": "Value"},
	}

	allowed, ruleId, err := rl.Allow(context.Background(), requestInfo)
	assert.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, "", ruleId)
}

func TestCalculateDelta2(t *testing.T) {
	storage := &mockStorage{}
	rl := ratelimiter.New(storage, 5, 1)
	//rl := &RateLimiter{}
	testCases := []struct {
		local         domain.Slots
		remote        domain.Slots
		lastSyncSlot  int
		curSlot       int
		expectedDelta int
	}{
		{
			local:         domain.Slots{0, 0, 5, 0, 0},
			remote:        domain.Slots{0, 0, 3, 0, 0},
			lastSyncSlot:  2,
			curSlot:       4,
			expectedDelta: 2,
		},
		{
			local:         domain.Slots{0, 0, 3, 0, 0},
			remote:        domain.Slots{0, 0, 5, 0, 0},
			lastSyncSlot:  2,
			curSlot:       4,
			expectedDelta: 0,
		},
		{
			local:         domain.Slots{2, 0, 0, 0, 0},
			remote:        domain.Slots{0, 0, 0, 0, 0},
			lastSyncSlot:  0,
			curSlot:       3,
			expectedDelta: 2,
		},
	}

	for _, tc := range testCases {
		delta := rl.CalculateDelta2(tc.local, tc.remote, tc.lastSyncSlot, tc.curSlot)
		if delta != tc.expectedDelta {
			t.Errorf("Failed for local=%v, remote=%v, lastSyncSlot=%d, curSlot=%d: expected delta=%d, got delta=%d", tc.local, tc.remote, tc.lastSyncSlot, tc.curSlot, tc.expectedDelta, delta)
		}
	}
}
