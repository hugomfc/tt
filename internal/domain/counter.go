package domain

type Slots []int

func NewSlots(rule *Rule, slotSize int) Slots {
	return make(Slots, rule.CountingPeriod/slotSize)
}

type Counter struct {
	Slots        Slots
	LastSyncSlot int
	LastIncrTime int64
	slotSize     int
	//  groupByKey string
	Rule *Rule
}

func NewCounter(rule *Rule, slotSize int) *Counter {
	return &Counter{
		slotSize:     slotSize,
		Slots:        make([]int, rule.CountingPeriod/slotSize),
		LastSyncSlot: -1,
		LastIncrTime: -1,
		Rule:         rule,
	}
}

func (c *Counter) CalculateSlot(time int64) int {
	return int(time / int64(c.slotSize) % int64(len(c.Slots)))
}
