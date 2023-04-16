package ratelimiter

import (
	"context"
	"fmt"
  "log"
	//"fmt"
	"sync"
	"time"

	"github.com/antonmedv/expr"
	"github.com/antonmedv/expr/vm"
	"github.com/hugomfc/tt/internal/domain"
	"github.com/hugomfc/tt/internal/storage"
)

type RateLimiter struct {
	storage       storage.Storage
	rules          map[string]*compiledRule
	rulesMutex     sync.RWMutex
	counters       map[string]map[string]*counter
	countersMutex  sync.RWMutex
	syncInterval   time.Duration
	slotSize       int
}

type compiledRule struct {
	*domain.Rule
	compiledExpr   *vm.Program
}

type counter struct {
  slots domain.Slots
	lastSyncSlot int 
  //  groupByKey string
  rule *domain.Rule
}

type RequestInfo struct {
	IP        string
	Method    string
	Path      string
	Headers   map[string]string
  // TODO: add more
}


func genGroupKey(groupBy []string, info *RequestInfo) *string {
  var groupByKey string

  for _, group := range groupBy {
    switch group {
    case "ip": groupByKey += info.IP
    case "path": groupByKey += info.Path
    case "method": groupByKey += info.Method
    // TODO: add more
    }
  }
  return &groupByKey
}

func New(storage storage.Storage, syncInterval time.Duration, slotSize int) *RateLimiter {
	rl := &RateLimiter{
		storage:       storage,
		rules:         make(map[string]*compiledRule),
		counters:      make(map[string]map[string]*counter),
		syncInterval:  syncInterval,
		slotSize:      slotSize,
	}
	go rl.synchronizeRules()
  go rl.synchronizeCounters()
	return rl
}

func (rl *RateLimiter) Allow(ctx context.Context, info *RequestInfo) (bool, string, error) {
  now := time.Now().Unix() // TODO: UnixNano in the future for subsecond slotSize
  log.Println("ALLOW", now, info)
	rl.rulesMutex.RLock()
	defer rl.rulesMutex.RUnlock()

	for _, rule := range rl.rules {
		allowed, err := rl.evaluateRule(ctx, rule, info, now)
		if err != nil {
			return false, "", err
		}
		if !allowed {
			return false, rule.ID, nil
		}
	}
	return true, "", nil
}

func (rl *RateLimiter) AddRule(ctx context.Context, rule *domain.Rule) error {
	compiledExpr, err := expr.Compile(rule.Expression)
	if err != nil {
		return err
	}

	compiledRule := &compiledRule{
		Rule:         rule,
		compiledExpr: compiledExpr,
	}

	rl.rulesMutex.Lock()
	rl.rules[rule.ID] = compiledRule
	rl.rulesMutex.Unlock()
  rl.countersMutex.Lock()
  rl.counters[rule.ID] = make(map[string]*counter)
  rl.countersMutex.Unlock()

	return rl.storage.AddRule(ctx, rule)
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
	ctx := context.Background()
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(rl.syncInterval):
			rules, err := rl.storage.GetRules(ctx)
			if err != nil {
				continue
			}
      for _, rule := range rules {
			  if !rl.HasRule(ctx, rule.ID) {
          rl.AddRule(ctx, rule)
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
  log.Println(slots)
}

func (rl *RateLimiter) synchronizeCounters() {
	ctx := context.Background()
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(rl.syncInterval):
      log.Println(rl.syncInterval, "now=", time.Now().Unix())
			rl.countersMutex.Lock()
      rl.rulesMutex.RLock()
      for _, cRule := range rl.rules {
        for groupByKey, c := range rl.counters[cRule.ID] {
          syncTime := time.Now().Unix() // TODO: UnixNano in the future for subsecond slotSize
          remoteCounters, err := rl.storage.GetCounter(ctx, cRule.Rule, groupByKey)
          if err != nil {
            panic(err)
          }
           

          log.Print("SYNC ")
          rl.printSlots(c.slots)
          log.Print("REDI ")
          rl.printSlots(*remoteCounters)

				  //delta := rl.calculateDelta(c, syncTime)
          delta := rl.calculateDelta2(c.slots, *remoteCounters, c.lastSyncSlot, rl.calculateSlot2(c.slots, syncTime))
          if delta==0 {
            continue
          } 
  
          log.Println("INCR", cRule.Rule, groupByKey, rl.calculateSlot(c, syncTime), delta)
          v, err := rl.storage.IncrementCounterBy(ctx, cRule.Rule, groupByKey, rl.calculateSlot(c, syncTime), delta)
				  if err != nil {
					  continue
				  }
          fmt.Println(v, err)
          if len(*remoteCounters)>cRule.WindowSize/int(rl.slotSize) {
            panic("unmatching WindowSize / slotSize")
          } 
          fmt.Println(len(*remoteCounters))
          for slot, slotCount := range *remoteCounters {
            c.slots[slot] = slotCount
          }
          c.slots[rl.calculateSlot(c, syncTime)] = v
          log.Print("2SYNC ")
          rl.printSlots(c.slots)


				  c.lastSyncSlot = rl.calculateSlot(c, syncTime)
        }
      }
      rl.rulesMutex.RUnlock()
			rl.countersMutex.Unlock()
		}
	}
}

func (rl *RateLimiter) evaluateRule(ctx context.Context, rule *compiledRule, info *RequestInfo, now int64) (bool, error) {
  groupByKey := genGroupKey(rule.GroupBy, info)
	rl.countersMutex.Lock()

	c, ok := rl.counters[rule.ID][*groupByKey]
	if !ok {
		c = &counter{
			lastSyncSlot: -1,
      slots:    make(domain.Slots, rule.WindowSize/int(rl.slotSize)),

      //groupByKey: groupByKey,
      rule: rule.Rule,
		}
		rl.counters[rule.ID][*groupByKey] = c
	}
  log.Println("calculateSlot=", rl.calculateSlot(c,now))
	c.slots[rl.calculateSlot(c, now)]++
  log.Println("EVAL")
  rl.printSlots(c.slots)
	rl.countersMutex.Unlock()


	sum := 0
	for _, count := range c.slots {
		sum += count
	}

	return sum <= rule.Limit, nil
}

func (rl *RateLimiter) calculateSlot(c *counter, t int64) int {
  return int(t / int64(rl.slotSize) % int64(len(c.slots)))
}

func (rl *RateLimiter) calculateSlot2(slots domain.Slots, t int64) int {
  return int(t / int64(rl.slotSize) % int64(len(slots)))
}

func (rl *RateLimiter) calculateDelta2(local domain.Slots, remote domain.Slots, lastSyncSlot int, curSlot int) int {
  delta := 0
  if lastSyncSlot>0 && local[lastSyncSlot]>remote[lastSyncSlot] {
    delta += local[lastSyncSlot]-remote[lastSyncSlot] 
  }
  delta += local[curSlot]
	local[curSlot] = 0
  //log.Println("<<<<", c.lastSyncSlot, lastSyncSlot, now, curSlot, delta)
	//for i := lastSyncSlot+1; i < curSlot; i++ {
  for i:=lastSyncSlot+1;i!=curSlot;i=(i+1)%len(local) {
		//slot := i % len(c.slots)
    slot := i
		delta += local[slot]
    local[slot] = 0
    //log.Println(">>", i, int(c.rule.WindowSize/rl.slotSize), now, rl.slotSize, slot, delta)
	}
	return delta
}



func (rl *RateLimiter) calculateDelta(c *counter, now int64) int {
  curSlot := rl.calculateSlot(c, now)
  lastSyncSlot := c.lastSyncSlot
  delta := c.slots[curSlot]
	c.slots[curSlot] = 0
  log.Println("<<<<", c.lastSyncSlot, lastSyncSlot, now, curSlot, delta)
	//for i := lastSyncSlot+1; i < curSlot; i++ {
  for i:=lastSyncSlot+1;i!=curSlot;i=(i+1)%len(c.slots) {
		//slot := i % len(c.slots)
    slot := i
		delta += c.slots[slot]
    c.slots[slot] = 0
    log.Println(">>", i, int(c.rule.WindowSize/rl.slotSize), now, rl.slotSize, slot, delta)
	}
	return delta
}

