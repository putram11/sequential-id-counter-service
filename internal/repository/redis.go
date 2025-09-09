package repository

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/putram11/sequential-id-counter-service/internal/config"
)

// RedisRepository handles Redis operations for counters
type RedisRepository struct {
	client redis.UniversalClient
}

// NewRedisRepository creates a new Redis repository
func NewRedisRepository(cfg config.RedisConfig) (*RedisRepository, error) {
	var client redis.UniversalClient
	
	if cfg.ClusterMode {
		// Parse cluster nodes from URL (simplified)
		client = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:    []string{cfg.URL},
			Password: cfg.Password,
		})
	} else {
		opt, err := redis.ParseURL(cfg.URL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
		}
		
		if cfg.Password != "" {
			opt.Password = cfg.Password
		}
		opt.DB = cfg.DB
		
		client = redis.NewClient(opt)
	}
	
	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}
	
	return &RedisRepository{
		client: client,
	}, nil
}

// IncrementCounter atomically increments a counter and returns the new value
func (r *RedisRepository) IncrementCounter(ctx context.Context, prefix string) (int64, error) {
	key := r.counterKey(prefix)
	result, err := r.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to increment counter for prefix %s: %w", prefix, err)
	}
	return result, nil
}

// GetCounter gets the current counter value
func (r *RedisRepository) GetCounter(ctx context.Context, prefix string) (int64, error) {
	key := r.counterKey(prefix)
	result, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return 0, nil // Counter doesn't exist, return 0
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get counter for prefix %s: %w", prefix, err)
	}
	
	counter, err := strconv.ParseInt(result, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse counter value: %w", err)
	}
	
	return counter, nil
}

// SetCounter sets the counter to a specific value
func (r *RedisRepository) SetCounter(ctx context.Context, prefix string, value int64) error {
	key := r.counterKey(prefix)
	err := r.client.Set(ctx, key, value, 0).Err()
	if err != nil {
		return fmt.Errorf("failed to set counter for prefix %s to %d: %w", prefix, value, err)
	}
	return nil
}

// IncrementCounterBy atomically increments a counter by a specific amount
func (r *RedisRepository) IncrementCounterBy(ctx context.Context, prefix string, increment int64) (int64, error) {
	key := r.counterKey(prefix)
	result, err := r.client.IncrBy(ctx, key, increment).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to increment counter for prefix %s by %d: %w", prefix, increment, err)
	}
	return result, nil
}

// GetMultipleCounters gets multiple counter values in a single operation
func (r *RedisRepository) GetMultipleCounters(ctx context.Context, prefixes []string) (map[string]int64, error) {
	if len(prefixes) == 0 {
		return make(map[string]int64), nil
	}
	
	// Prepare keys
	keys := make([]string, len(prefixes))
	for i, prefix := range prefixes {
		keys[i] = r.counterKey(prefix)
	}
	
	// Use pipeline for efficiency
	pipe := r.client.Pipeline()
	cmds := make([]*redis.StringCmd, len(keys))
	for i, key := range keys {
		cmds[i] = pipe.Get(ctx, key)
	}
	
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to get multiple counters: %w", err)
	}
	
	// Parse results
	result := make(map[string]int64)
	for i, cmd := range cmds {
		val, err := cmd.Result()
		if err == redis.Nil {
			result[prefixes[i]] = 0
		} else if err != nil {
			return nil, fmt.Errorf("failed to get counter for prefix %s: %w", prefixes[i], err)
		} else {
			counter, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse counter value for prefix %s: %w", prefixes[i], err)
			}
			result[prefixes[i]] = counter
		}
	}
	
	return result, nil
}

// Ping checks if Redis is accessible
func (r *RedisRepository) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// Close closes the Redis connection
func (r *RedisRepository) Close() error {
	return r.client.Close()
}

// counterKey generates the Redis key for a counter
func (r *RedisRepository) counterKey(prefix string) string {
	return fmt.Sprintf("seq:%s", prefix)
}

// ResetCounter resets a counter to a specific value (used for admin operations)
func (r *RedisRepository) ResetCounter(ctx context.Context, prefix string, newValue int64) (int64, error) {
	key := r.counterKey(prefix)
	
	// Use a transaction to get old value and set new value atomically
	var oldValue int64
	err := r.client.Watch(ctx, func(tx *redis.Tx) error {
		// Get current value
		val, err := tx.Get(ctx, key).Result()
		if err == redis.Nil {
			oldValue = 0
		} else if err != nil {
			return err
		} else {
			oldValue, err = strconv.ParseInt(val, 10, 64)
			if err != nil {
				return err
			}
		}
		
		// Set new value
		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.Set(ctx, key, newValue, 0)
			return nil
		})
		return err
	}, key)
	
	if err != nil {
		return 0, fmt.Errorf("failed to reset counter for prefix %s: %w", prefix, err)
	}
	
	return oldValue, nil
}

// GetInfo returns Redis information for monitoring
func (r *RedisRepository) GetInfo(ctx context.Context) (map[string]string, error) {
	info, err := r.client.Info(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get Redis info: %w", err)
	}
	
	// Parse info string into map (simplified)
	result := make(map[string]string)
	result["raw_info"] = info
	
	return result, nil
}
