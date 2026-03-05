package store

import (
	"KVStore/limiter"
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

/*
并发安全的内存KV Store
功能要求：
Get(key) / Set(key, value) / Delete(key)
支持TTL过期（key到期自动删除）
支持并发读写（读多写少场景）
有基本的访问限流（用rate limiter）
优雅关闭（用context）
*/

type LockStrategy string

const (
	LockStrategyGlobal  LockStrategy = "global"
	LockStrategySharded LockStrategy = "sharded"

	defaultShardCount = 16
)

type KVStore struct {
	data map[string]entry
	mu   sync.RWMutex

	shards       []kvShard
	lockStrategy LockStrategy

	limiter *limiter.RateLimiter

	totalWriteLockWaitNs atomic.Int64
	writeLockWaitSamples atomic.Int64
}

type kvShard struct {
	data map[string]entry
	mu   sync.RWMutex
}

type entry struct {
	value     string
	expiresAt time.Time
}

var tracer = otel.Tracer("kvstore/store")

var errRateLimitExceeded = errors.New("rate limit exceed!")

// constructor
func New(ctx context.Context, cleanupInterval time.Duration, rate time.Duration, capacity int) *KVStore {
	return NewWithLockStrategy(ctx, cleanupInterval, rate, capacity, LockStrategyGlobal, 1)
}

func NewWithLockStrategy(
	ctx context.Context,
	cleanupInterval time.Duration,
	rate time.Duration,
	capacity int,
	strategy LockStrategy,
	shardCount int,
) *KVStore {
	rl := limiter.NewLimiter(ctx, rate, capacity)
	s := &KVStore{
		limiter: rl,
	}

	if strategy == LockStrategySharded {
		if shardCount <= 0 {
			shardCount = defaultShardCount
		}
		s.lockStrategy = LockStrategySharded
		s.shards = make([]kvShard, shardCount)
		for i := range s.shards {
			s.shards[i] = kvShard{
				data: make(map[string]entry),
			}
		}
	} else {
		s.lockStrategy = LockStrategyGlobal
		s.data = make(map[string]entry)
	}

	// run the background cleanup
	go s.startCleanup(ctx, cleanupInterval)

	return s
}

func (s *KVStore) Strategy() LockStrategy {
	return s.lockStrategy
}

func (s *KVStore) ShardCount() int {
	if s.lockStrategy != LockStrategySharded {
		return 1
	}
	return len(s.shards)
}

// 3 APIs
// func (接收者) 方法名(参数列表) 返回值列表 { ... }
// 接收者必须传指针，接受者为s，go的函数返回值可以是多个

// Get
// lazy expiration + 后台cleanup
func (s *KVStore) Get(ctx context.Context, key string) (string, bool, error) {
	ctx, span := tracer.Start(ctx, "kvstore.get")
	defer span.End()
	span.SetAttributes(
		attribute.String("kv.key", key),
		attribute.String("kv.lock.strategy", string(s.lockStrategy)),
	)

	if !s.limiter.Allow(ctx) {
		span.RecordError(errRateLimitExceeded)
		span.SetStatus(codes.Error, errRateLimitExceeded.Error())
		span.SetAttributes(attribute.Bool("kv.rate_limited", true))
		return "", false, errRateLimitExceeded
	}
	span.SetAttributes(attribute.Bool("kv.rate_limited", false))

	var e entry
	var ok bool

	if s.lockStrategy == LockStrategySharded {
		shardIdx := s.shardIndex(key)
		shard := &s.shards[shardIdx]
		span.SetAttributes(attribute.Int("kv.shard.index", shardIdx))
		shard.mu.RLock()
		e, ok = shard.data[key]
		shard.mu.RUnlock()
	} else {
		s.mu.RLock()
		e, ok = s.data[key]
		s.mu.RUnlock()
	}

	// 没找到
	if !ok {
		span.SetAttributes(attribute.Bool("kv.found", false))
		return "", false, nil
	}
	// 过期
	if time.Now().After(e.expiresAt) {
		span.SetAttributes(
			attribute.Bool("kv.found", false),
			attribute.Bool("kv.expired", true),
		)
		return "", false, nil
	}

	span.SetAttributes(
		attribute.Bool("kv.found", true),
		attribute.Bool("kv.expired", false),
	)
	return e.value, true, nil
}

// Set
func (s *KVStore) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	ctx, span := tracer.Start(ctx, "kvstore.set")
	defer span.End()
	span.SetAttributes(
		attribute.String("kv.key", key),
		attribute.Int64("kv.ttl_ms", ttl.Milliseconds()),
		attribute.String("kv.lock.strategy", string(s.lockStrategy)),
	)

	if !s.limiter.Allow(ctx) {
		span.RecordError(errRateLimitExceeded)
		span.SetStatus(codes.Error, errRateLimitExceeded.Error())
		span.SetAttributes(attribute.Bool("kv.rate_limited", true))
		return errRateLimitExceeded
	}
	span.SetAttributes(attribute.Bool("kv.rate_limited", false))

	expiresAt := time.Now().Add(ttl)

	if s.lockStrategy == LockStrategySharded {
		shardIdx := s.shardIndex(key)
		waitDuration := s.waitForShardWriteLock(ctx, "set", shardIdx)
		span.SetAttributes(
			attribute.Int("kv.shard.index", shardIdx),
			attribute.Int64("kv.mutex.wait_ns", waitDuration.Nanoseconds()),
			attribute.Float64("kv.mutex.wait_ms", float64(waitDuration)/float64(time.Millisecond)),
		)
		shard := &s.shards[shardIdx]
		defer shard.mu.Unlock()
		shard.data[key] = entry{value: value, expiresAt: expiresAt}
		return nil
	}

	waitDuration := s.waitForWriteLock(ctx, "set")
	span.SetAttributes(
		attribute.Int64("kv.mutex.wait_ns", waitDuration.Nanoseconds()),
		attribute.Float64("kv.mutex.wait_ms", float64(waitDuration)/float64(time.Millisecond)),
	)
	defer s.mu.Unlock()
	s.data[key] = entry{value: value, expiresAt: expiresAt}
	return nil
}

// Delete
func (s *KVStore) Delete(ctx context.Context, key string) error {
	ctx, span := tracer.Start(ctx, "kvstore.delete")
	defer span.End()
	span.SetAttributes(
		attribute.String("kv.key", key),
		attribute.String("kv.lock.strategy", string(s.lockStrategy)),
	)

	if !s.limiter.Allow(ctx) {
		span.RecordError(errRateLimitExceeded)
		span.SetStatus(codes.Error, errRateLimitExceeded.Error())
		span.SetAttributes(attribute.Bool("kv.rate_limited", true))
		return errRateLimitExceeded
	}
	span.SetAttributes(attribute.Bool("kv.rate_limited", false))

	if s.lockStrategy == LockStrategySharded {
		shardIdx := s.shardIndex(key)
		waitDuration := s.waitForShardWriteLock(ctx, "delete", shardIdx)
		span.SetAttributes(
			attribute.Int("kv.shard.index", shardIdx),
			attribute.Int64("kv.mutex.wait_ns", waitDuration.Nanoseconds()),
			attribute.Float64("kv.mutex.wait_ms", float64(waitDuration)/float64(time.Millisecond)),
		)
		shard := &s.shards[shardIdx]
		defer shard.mu.Unlock()
		delete(shard.data, key)
		return nil
	}

	waitDuration := s.waitForWriteLock(ctx, "delete")
	span.SetAttributes(
		attribute.Int64("kv.mutex.wait_ns", waitDuration.Nanoseconds()),
		attribute.Float64("kv.mutex.wait_ms", float64(waitDuration)/float64(time.Millisecond)),
	)
	defer s.mu.Unlock()
	delete(s.data, key)
	return nil
}

// Interval cleanup to avoid memory leak
func (s *KVStore) startCleanup(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	// 在for {}循环里永远不要直接defer锁
	for {
		select {
		case <-ctx.Done():
			fmt.Printf("Off work!")
			return

		case <-ticker.C:
			// start cleanup
			s.cleanupExpired(ctx)
		}
	}
}

// 抽离出ticker.C里的逻辑
func (s *KVStore) cleanupExpired(ctx context.Context) {
	_, span := tracer.Start(ctx, "kvstore.cleanup_expired")
	defer span.End()

	now := time.Now()
	deleted := 0

	if s.lockStrategy == LockStrategySharded {
		totalWait := time.Duration(0)
		for shardIdx := range s.shards {
			waitDuration := s.waitForShardWriteLock(ctx, "cleanup", shardIdx)
			totalWait += waitDuration
			shard := &s.shards[shardIdx]

			for k, v := range shard.data {
				if now.After(v.expiresAt) {
					delete(shard.data, k)
					deleted++
				}
			}
			shard.mu.Unlock()
		}

		span.SetAttributes(
			attribute.String("kv.lock.strategy", string(s.lockStrategy)),
			attribute.Int("kv.shard.count", len(s.shards)),
			attribute.Int("kv.cleanup.deleted", deleted),
			attribute.Int64("kv.mutex.wait_ns", totalWait.Nanoseconds()),
			attribute.Float64("kv.mutex.wait_ms", float64(totalWait)/float64(time.Millisecond)),
		)
		return
	}

	waitDuration := s.waitForWriteLock(ctx, "cleanup")
	defer s.mu.Unlock()

	for k, v := range s.data {
		if now.After(v.expiresAt) {
			delete(s.data, k)
			deleted++
		}
	}

	span.SetAttributes(
		attribute.String("kv.lock.strategy", string(s.lockStrategy)),
		attribute.Int("kv.cleanup.deleted", deleted),
		attribute.Int64("kv.mutex.wait_ns", waitDuration.Nanoseconds()),
		attribute.Float64("kv.mutex.wait_ms", float64(waitDuration)/float64(time.Millisecond)),
	)
}

func (s *KVStore) waitForWriteLock(ctx context.Context, operation string) time.Duration {
	return s.measureWriteLockWait(ctx, operation, "global", -1, func() {
		s.mu.Lock()
	})
}

func (s *KVStore) waitForShardWriteLock(ctx context.Context, operation string, shardIdx int) time.Duration {
	return s.measureWriteLockWait(ctx, operation, "shard", shardIdx, func() {
		s.shards[shardIdx].mu.Lock()
	})
}

func (s *KVStore) measureWriteLockWait(
	ctx context.Context,
	operation string,
	lockScope string,
	shardIdx int,
	lockFn func(),
) time.Duration {
	_, waitSpan := tracer.Start(ctx, "kvstore.mutex.wait")
	waitSpan.SetAttributes(
		attribute.String("kv.mutex.type", "write"),
		attribute.String("kv.lock.scope", lockScope),
		attribute.String("kv.operation", operation),
		attribute.String("kv.lock.strategy", string(s.lockStrategy)),
	)
	if shardIdx >= 0 {
		waitSpan.SetAttributes(attribute.Int("kv.shard.index", shardIdx))
	}

	waitStart := time.Now()
	lockFn()
	waitDuration := time.Since(waitStart)

	waitSpan.SetAttributes(
		attribute.Int64("kv.mutex.wait_ns", waitDuration.Nanoseconds()),
		attribute.Float64("kv.mutex.wait_ms", float64(waitDuration)/float64(time.Millisecond)),
	)
	waitSpan.End()

	s.totalWriteLockWaitNs.Add(waitDuration.Nanoseconds())
	s.writeLockWaitSamples.Add(1)
	return waitDuration
}

func (s *KVStore) WriteLockWaitStats() (total time.Duration, avg time.Duration, samples int64) {
	totalNs := s.totalWriteLockWaitNs.Load()
	samples = s.writeLockWaitSamples.Load()
	total = time.Duration(totalNs)
	if samples == 0 {
		return total, 0, 0
	}

	avg = time.Duration(totalNs / samples)
	return total, avg, samples
}

func (s *KVStore) shardIndex(key string) int {
	if len(s.shards) == 0 {
		return 0
	}

	var hash uint32 = 2166136261
	for i := 0; i < len(key); i++ {
		hash ^= uint32(key[i])
		hash *= 16777619
	}
	return int(hash % uint32(len(s.shards)))
}
