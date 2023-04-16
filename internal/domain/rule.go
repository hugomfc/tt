package domain

import (
  "errors"
)

type Rule struct {
	ID          string
	Expression  string
	GroupBy     []string
	Limit       int
  WindowSize  int // in seconds, e.g, for a rate limit of 1000 rpm, the window size should be 60s
}

var (
	ErrRuleNotFound = errors.New("rule not found")
)
