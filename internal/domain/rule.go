package domain

import (
	"errors"
)

type Rule struct {
	ID             string
	Expression     string
	GroupBy        []string
	Limit          int
	//WindowSize     int // in seconds, e.g, for a rate limit of 1000 rpm, the window size should be 60s
	CountingPeriod int // 10s, 60s (1m), 3600 (1h), 8600 (1d)
}

var (
	ErrRuleNotFound = errors.New("rule not found")
)
