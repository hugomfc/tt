package ratelimiter

import (
	"context"
	"errors"
	"log"

	//"fmt"
	"sync"
	"time"

	"github.com/antonmedv/expr"
	"github.com/antonmedv/expr/vm"
	"github.com/hugomfc/tt/internal/domain"
	"github.com/hugomfc/tt/internal/storage"
)

var (
	ErrCountingPeriodTooShort = errors.New("counting period too short for the defined sync interval")
)

type RateLimiter struct {
	storage       storage.Storage
	rules         map[string]*CompiledRule
	rulesMutex    sync.RWMutex
	counters      map[string]map[string]*domain.Counter
	countersMutex sync.RWMutex
	syncInterval  time.Duration
	SlotSize      int // in seconds

	// handle sync goroutines and shufdown
	ctxSync    context.Context
	cancelSync context.CancelFunc
	wgSync     sync.WaitGroup
}

type CompiledRule struct {
	*domain.Rule
	compiledExpr *vm.Program
}

type RequestInfo struct {
	IP      string
	Method  string
	Path    string
	Headers map[string]string
	// TODO: add more
}

func genGroupKey(groupBy []string, info *RequestInfo) *string {
	var groupByKey string

	for _, group := range groupBy {
		switch group {
		case "ip":
			groupByKey += info.IP
		case "path":
			groupByKey += info.Path
		case "method":
			groupByKey += info.Method
			// TODO: add more
		}
	}
	return &groupByKey
}

func New(storage storage.Storage, syncInterval time.Duration, slotSize int) *RateLimiter {
	ctx, cancel := context.WithCancel(context.Background())

	rl := &RateLimiter{
		storage:      storage,
		rules:        make(map[string]*CompiledRule),
		counters:     make(map[string]map[string]*domain.Counter),
		syncInterval: syncInterval,
		SlotSize:     slotSize,

		ctxSync:    ctx,
		cancelSync: cancel,
		//wgSync:   sync.WaitGroup{},

	}
	rl.wgSync.Add(2)

	go func() {
		defer rl.wgSync.Done()
		rl.synchronizeRules()
	}()

	go func() {
		defer rl.wgSync.Done()
		rl.synchronizeCounters()
	}()

	return rl
}

func (rl *RateLimiter) Shutdown() {
	rl.cancelSync()
	rl.wgSync.Wait()
	log.Println("Rate limiter has been shutdown")
}

func (rl *RateLimiter) Allow(ctx context.Context, info *RequestInfo) (bool, string, error) {
	now := time.Now().Unix() // TODO: UnixNano in the future for subsecond slotSize
	rl.rulesMutex.RLock()
	defer rl.rulesMutex.RUnlock()

	for _, rule := range rl.rules {
		output, err := expr.Run(rule.compiledExpr, info)
		if err != nil {
			return false, "", err
		}
		if output.(bool) {
			allowed, err := rl.evaluateRule(ctx, rule, info, now)
			if err != nil {
				return false, "", err
			}
			if !allowed {
				return false, rule.ID, nil
			}
		}
	}
	return true, "", nil
}

// validate counting period is at least X times the sync interval. This is to make sure
// that the sync goroutine cleans up the old slots
func (rl *RateLimiter) isCountingPeriodOk(rule *domain.Rule) bool {
	//log.Println(time.Duration(rule.CountingPeriod*int(time.Second)), rl.syncInterval*3)
	return time.Duration(rule.CountingPeriod*int(time.Second)) >= rl.syncInterval*3
}

func (rl *RateLimiter) AddRule(ctx context.Context, rule domain.Rule) error {
	if !rl.isCountingPeriodOk(&rule) {
		return ErrCountingPeriodTooShort
	}

	compiledExpr, err := expr.Compile(rule.Expression)
	if err != nil {
		return err
	}

	compiledRule := &CompiledRule{
		Rule:         &rule,
		compiledExpr: compiledExpr,
	}

	rl.rulesMutex.Lock()
	rl.rules[rule.ID] = compiledRule
	rl.rulesMutex.Unlock()
	rl.countersMutex.Lock()
	rl.counters[rule.ID] = make(map[string]*domain.Counter)
	rl.countersMutex.Unlock()

	return rl.storage.AddRule(ctx, &rule)
}

func (rl *RateLimiter) HasRule(ctx context.Context, ruleID string) bool {
	rl.rulesMutex.RLock()
	defer rl.rulesMutex.RUnlock()
	_, hasRule := rl.rules[ruleID]
	return hasRule
}

func (rl *RateLimiter) DeleteRule(ctx context.Context, ruleID string) error {
	rl.rulesMutex.Lock()
	delete(rl.rules, ruleID)
	rl.rulesMutex.Unlock()

	return rl.storage.DeleteRule(ctx, ruleID)
}

func (rl *RateLimiter) synchronizeRules() {
	for {
		select {
		case <-rl.ctxSync.Done():
			return
		case <-time.After(rl.syncInterval):
			rules, err := rl.storage.GetRules(rl.ctxSync)
			if err != nil {
				continue
			}
			for _, rule := range rules {
				if !rl.HasRule(rl.ctxSync, rule.ID) {
					rl.AddRule(rl.ctxSync, *rule)
				}
			}

		}
	}
}

func (rl *RateLimiter) printSlots(slots domain.Slots) {
	//for i:=0; i<len(slots); i++ {
	//  log.Printf("%d:%d", i, slots[i])
	//}
	//log.Printf("\n")
	log.Println(">>>>>>", slots)
}

func (rl *RateLimiter) printSlotsSync(slots domain.Slots) {
	//for i:=0; i<len(slots); i++ {
	//  log.Printf("%d:%d", i, slots[i])
	//}
	//log.Printf("\n")
	log.Println("SYNC<<<<", slots)
}

func (rl *RateLimiter) synchronizeCounters() {
	for {
		select {
		case <-rl.ctxSync.Done():
			return
		case <-time.After(rl.syncInterval):
			//log.Println("SYNCING COUNTERS")
			countersToDelete := make([]string, len(rl.counters)/3) // TODO: come up with a better default size for this
			rl.countersMutex.Lock()
			rl.rulesMutex.RLock()
			for _, cRule := range rl.rules {
				//log.Println("RULE ID", cRule.ID)
				for groupByKey, c := range rl.counters[cRule.ID] {
					syncTime := time.Now().Unix()
					remoteCounter, errRemote := rl.storage.GetCounter(rl.ctxSync, rl.SlotSize, cRule.Rule, groupByKey)

					if errRemote == storage.ErrCounterNotFound {
						remoteCounter = domain.NewCounter(cRule.Rule, rl.SlotSize)
					}
					//rl.printSlots(c.Slots)
					//rl.printSlots(remoteCounter.Slots)

					delta := rl.CalculateDelta2(c.Slots, remoteCounter.Slots, c.LastSyncSlot, c.CalculateSlot(syncTime))
					if errRemote == storage.ErrCounterNotFound && delta == 0 {
						//log.Println("DELTA IS 0 AND REMOTE COUNTER NOT FOUND")
						countersToDelete = append(countersToDelete, groupByKey)
						continue
					}
					//log.Println("DELTA", delta)
					if delta > 0 {
						newCount, errRemote := rl.storage.IncrementCounterBy(rl.ctxSync, cRule.Rule, groupByKey, c.CalculateSlot(syncTime), delta)
						if errRemote != nil {
							continue
						}
						if len(remoteCounter.Slots) > cRule.CountingPeriod/int(rl.SlotSize) {
							panic("unmatching WindowSize / slotSize")
						}
						copy(c.Slots, remoteCounter.Slots)
						c.Slots[c.CalculateSlot(syncTime)] = newCount
					} else {
						copy(c.Slots, remoteCounter.Slots)
					}
					c.LastSyncSlot = c.CalculateSlot(syncTime)
					rl.printSlotsSync(c.Slots)
					rl.printSlotsSync(remoteCounter.Slots)

				}
				for _, groupByKey := range countersToDelete {
					//log.Println("DELETING COUNTER", groupByKey)

					delete(rl.counters[cRule.ID], groupByKey)
				}
			}
			rl.rulesMutex.RUnlock()
			rl.countersMutex.Unlock()

		}
		//log.Println("FINISHEDSYNCING COUNTERS")
	}
}

func (rl *RateLimiter) evaluateRule(ctx context.Context, rule *CompiledRule, info *RequestInfo, now int64) (bool, error) {
	groupByKey := genGroupKey(rule.GroupBy, info)
	rl.countersMutex.Lock()
	c, ok := rl.counters[rule.ID][*groupByKey]
	if !ok {
		rl.counters[rule.ID][*groupByKey] = domain.NewCounter(rule.Rule, rl.SlotSize)
		c = rl.counters[rule.ID][*groupByKey]
	}
	rl.printSlots(c.Slots)

	slot := c.CalculateSlot(now)

	log.Println("calculateSlot=", slot)

	c.Slots[slot]++
	log.Println("EVAL RULE", rule.ID)
	//rl.printSlots(c.Slots)
	rl.countersMutex.Unlock()

	sum := 0
	for _, count := range c.Slots {
		sum += count
	}
	rl.printSlots(c.Slots)

	return sum <= rule.Limit, nil
}

func (rl *RateLimiter) CalculateDelta2(local domain.Slots, remote domain.Slots, lastSyncSlot int, curSlot int) int {
	delta := 0
	//log.Println("CALCULATE DELTA", lastSyncSlot, curSlot)
	if lastSyncSlot >= 0 {
		if local[lastSyncSlot] > remote[lastSyncSlot] {
			delta += local[lastSyncSlot] - remote[lastSyncSlot]
		} else {
			delta = 0
		}
	}
	delta += local[curSlot]
	local[curSlot] = 0
	for i := (lastSyncSlot + 1) % len(local); i != curSlot; i = (i + 1) % len(local) {
		slot := i
		delta += local[slot]
		local[slot] = 0
	}
	return delta
}

func (rl *RateLimiter) calculateDelta(c *domain.Counter, now int64) int {
	curSlot := c.CalculateSlot(now)
	lastSyncSlot := c.LastSyncSlot
	delta := c.Slots[curSlot]
	c.Slots[curSlot] = 0
	//log.Println("<<<<", c.LastSyncSlot, lastSyncSlot, now, curSlot, delta)
	//for i := lastSyncSlot+1; i < curSlot; i++ {
	for i := lastSyncSlot + 1; i != curSlot; i = (i + 1) % len(c.Slots) {
		//slot := i % len(c.slots)
		slot := i
		delta += c.Slots[slot]
		c.Slots[slot] = 0
		//log.Println(">>", i, int(c.Rule.CountingPeriod/rl.SlotSize), now, rl.SlotSize, slot, delta)
	}
	return delta
}
