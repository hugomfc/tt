package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/hugomfc/tt/internal/domain"
)

type RedisStorage struct {
	client     *redis.Client
	counterTTL time.Duration
}

func NewRedis(addr, password string, db int, counterTTL time.Duration) (*RedisStorage, error) {
  client := redis.NewClient(&redis.Options{
    Addr:     addr,
    Password: password,
    DB:       db,
  })

  err := client.Ping(context.Background()).Err()
  if err != nil {
    return nil, err
  }
	
  return &RedisStorage{
    client: client,
    counterTTL: counterTTL,
  }, nil
}

func (rs *RedisStorage) AddRule(ctx context.Context, rule *domain.Rule) error {
	data, err := json.Marshal(rule)
	if err != nil {
		return err
	}
	return rs.client.HSet(ctx, "rules", rule.ID, data).Err()
}

func (rs *RedisStorage) UpdateRule(ctx context.Context, rule *domain.Rule) error {
	return rs.AddRule(ctx, rule)
}

func (rs *RedisStorage) DeleteRule(ctx context.Context, key string) error {
	return rs.client.HDel(ctx, "rules", key).Err()
}

func (rs *RedisStorage) GetRules(ctx context.Context) ([]*domain.Rule, error) {
	rulesData, err := rs.client.HGetAll(ctx, "rules").Result()
	if err != nil {
		return nil, err
	}

	var rules []*domain.Rule
	for _, data := range rulesData {
		var rule domain.Rule
		if err := json.Unmarshal([]byte(data), &rule); err != nil {
			return nil, err
		}
		rules = append(rules, &rule)
	}
	return rules, nil
}

func (r *RedisStorage) GetCounter(ctx context.Context, rule *domain.Rule, groupBy string) (*domain.Slots, error) {
  counter := make(domain.Slots, rule.WindowSize)
  redisKey := fmt.Sprintf("counters:%s:%s", rule.ID, groupBy)
  log.Println("redisKey", redisKey)

	redisCounter, err := r.client.HGetAll(ctx, redisKey).Result()
	if err != nil {
		return nil, err
	}

  if len(redisCounter) == 0 { // doesn't exist
    return &counter, nil
  }

  for i:=0 ; i<rule.WindowSize; i++ {
    slotStr := strconv.Itoa(i)
    slotCount, ok := redisCounter[slotStr]

    if ok {
      counter[i],err = strconv.Atoi(slotCount)
      if err!=nil {
        panic("issue with slot conversion")
      }
    }
  }

  return &counter, nil
}

func (r *RedisStorage) IncrementCounterBy(ctx context.Context, rule *domain.Rule, groupBy string, slot int, delta int) (int, error) {

  redisKey := fmt.Sprintf("counters:%s:%s", rule.ID, groupBy)
  slotKey := fmt.Sprintf("%d", slot)

	pipe := r.client.TxPipeline()
	incr := pipe.HIncrBy(ctx, redisKey , slotKey, int64(delta))

	// Update counter's TTL to mark the counter was used
  pipe.PExpire(ctx, redisKey, r.counterTTL)
  _, err := pipe.Exec(ctx)
	if err != nil {
		return 0, err
	}
  fmt.Println("increment: ", redisKey, slotKey, delta, incr.Val(), r.counterTTL)
	return int(incr.Val()), nil
}
