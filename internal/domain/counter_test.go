package domain

import (
	"testing"
)

func TestCalculateSlot(t *testing.T) {
	counter := &Counter{
		slotSize: 10,
		Slots:    make(Slots, 3),
	}

	tests := []struct {
		time     int64
		expected int
	}{
		{0, 0},         // time = 0 / 10 % 3 = 0
		{5, 0},         // time = 5 / 10 % 3 = 0
		{10, 1},        // time = 10 / 10 % 3 = 1
		{15, 1},        // time = 15 / 10 % 3 = 1
		{20, 2},        // time = 20 / 10 % 3 = 2
		{25, 2},        // time = 25 / 10 % 3 = 2
		{30, 0},        // time = 30 / 10 % 3 = 0
		{100, 1},       // time = 100 / 10 % 3 = 1
		{123456789, 0}, // time = 123456789 / 10 % 3 = 0
	}

	for _, test := range tests {
		slot := counter.CalculateSlot(test.time)
		if slot != test.expected {
			t.Errorf("For time=%d, expected slot=%d, but got slot=%d", test.time, test.expected, slot)
		}
	}
}
